package sharef

import (
	"reflect"
	"sync"
)

// Sharef is a shared reference; copies of a Sharef always refer to
// the same value, so a modification to any copy implies a state
// mutation across all copies.
type Sharef[T any] struct {
	state **T
	name  *string
	group *Group[T]
}

// New() creates a new Sharef;
// New *panics* if:
// 1: a pointer is provided as its value.
func New[T any](value T) Sharef[T] {
	// Prevent pointers during runtime.
	reflectedValue := reflect.ValueOf(value)
	if reflectedValue.Kind() == reflect.Ptr {
		panic("Invalid state: pointer was provided.")
	}

	pointer := &value
	instance := Sharef[T]{
		state: &pointer,
	}

	return instance
}

// Do applies a given function to the Sharef's value;
// It creates a Portal for reading and writing the current and
// modified values, executes the provided function with the Portal and
// updates the Sharef's state based on the modifications;
// Do *panics* if:
// 1: the Sharef's value was never originally provided (zero value);
// 2: if a previous Do() call set the value to nil;
// *Note*: Do *is not atomic*, for atomicity to be guaranteed, please use a
// mutex;
func (this Sharef[T]) Do(body func(Portal[T])) {
	if this.state == nil || *this.state == nil {
		panic("Invalid state: value is nil.")
	}

	reader := make(chan *T)
	writer := make(chan *T)
	portal := Portal[T]{
		Reader: reader,
		Writer: writer,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		body(portal)
		wg.Done()
	}()

	previous := *this.state
	reader <- previous
	close(reader)

	current := <-writer
	*this.state = current
	close(writer)

	if this.group != nil && this.name != nil {
		this.group.doReadWrite(*this.name, previous, current)
	}

	wg.Wait()
}
