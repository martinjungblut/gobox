package atom

import (
	"reflect"
	"sync"
)

// Atom is a shared reference; copies of an Atom always refer to the
// same value, so a modification to any copy implies a state mutation
// across all copies.
type Atom[T any] struct {
	state **T
	name  *string
	group *AtomGroup[T]
}

// New() creates a new Atom.
func New[T any](value T) Atom[T] {
	pointer := &value
	instance := Atom[T]{
		state: &pointer,
	}

	// Prevent pointers during runtime.
	reflectedValue := reflect.ValueOf(value)
	if reflectedValue.Kind() == reflect.Ptr {
		// Die.
		*instance.state = nil
	}

	return instance
}

// Dead() creates a dead Atom; it replaces the uses of nil pointers
// when we want to represent optionality.
func Dead[T any]() Atom[T] {
	var value T
	var pointer *T = nil

	instance := New(value)
	instance.state = &pointer

	return instance
}

// Do applies a given function to the Atom's value within a
// synchronized environment;
// It locks the provided locker, creates a Portal for reading and
// writing the current and modified values, executes the provided
// function with the Portal, updates the Atom's state based on the
// modifications, and releases the lock;
// If the Atom is dead (has a nil pointer), the function is not
// executed;
// Note: The provided locker should be the same for all concurrent
// operations on the Atom to ensure proper synchronization.
func (this Atom[T]) Do(locker sync.Locker, body func(Portal[T])) {
	if this.IsDead() {
		return
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

	locker.Lock()
	previous := *this.state
	reader <- previous
	close(reader)

	current := <-writer
	*this.state = current
	close(writer)

	if this.group != nil && this.name != nil {
		this.group.DoReadWrite(*this.name, previous, current)
	}
	locker.Unlock()

	wg.Wait()
}

func (this Atom[T]) IsAlive() bool {
	return *this.state != nil
}

func (this Atom[T]) IsDead() bool {
	return *this.state == nil
}
