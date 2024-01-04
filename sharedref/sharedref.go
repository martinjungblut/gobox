package sharedref

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

// SharedRef is a shared reference; copies of a SharedRef always refer
// to the same value, so a modification to any copy implies a state
// mutation across all copies.
type SharedRef[T any] struct {
	state **T
}

// New() creates a new SharedRef.
func New[T any](value T) SharedRef[T] {
	pointer := &value
	instance := SharedRef[T]{
		state: &pointer,
	}

	// Prevent pointers during runtime.
	rvalue := reflect.ValueOf(value)
	if rvalue.Kind() == reflect.Ptr {
		// Die.
		*instance.state = nil
	}

	return instance
}

// Dead() creates a dead SharedRef; it replaces the uses of nil
// pointers when we want to represent optionality.
func Dead[T any]() SharedRef[T] {
	var value T
	var pointer *T = nil

	instance := New(value)
	instance.state = &pointer

	return instance
}

func (this SharedRef[T]) Do(locker sync.Locker, body func(Portal[T])) {
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
		locker.Lock()
		body(portal)
		wg.Done()
		locker.Unlock()
	}()

	reader <- *this.state
	close(reader)

	*this.state = <-writer
	close(writer)

	wg.Wait()
}

func (this SharedRef[T]) IsAlive() bool {
	return *this.state != nil
}

func (this SharedRef[T]) IsDead() bool {
	return *this.state == nil
}
