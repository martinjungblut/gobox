package atom

import (
	"reflect"
	"sync"
)

// Portal is a communication bridge that facilitates interaction
// between two distinct parts of the code. It provides a Reader
// channel for receiving values and a Writer channel for sending
// values. This allows seamless communication and data exchange
// between different components or goroutines.
type Portal[T any] struct {
	Reader <-chan *T
	Writer chan<- *T
}

// Atom is a shared reference; copies of an Atom always refer to the
// same value, so a modification to any copy implies a state mutation
// across all copies.
type Atom[T any] struct {
	state **T
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

	if this.group != nil {
		this.group.DoReadWrite(previous, current)
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

type AtomGroup[T any] struct {
	name        string
	onReadWrite func(string, *T, *T)
	mutex       sync.Mutex
}

func NewAtomGroup[T any](name string) AtomGroup[T] {
	return AtomGroup[T]{
		name:  name,
		mutex: sync.Mutex{},
	}
}

func (this *AtomGroup[T]) New(value T) Atom[T] {
	atom := New(value)
	atom.group = this
	return atom
}

func (this *AtomGroup[T]) Dead() Atom[T] {
	atom := Dead[T]()
	atom.group = this
	return atom
}

func (this *AtomGroup[T]) OnReadWrite(callback func(string, *T, *T)) {
	this.onReadWrite = callback
}

func (this *AtomGroup[T]) DoReadWrite(previous *T, current *T) {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	if this.onReadWrite != nil {
		this.onReadWrite(this.name, previous, current)
	}
}
