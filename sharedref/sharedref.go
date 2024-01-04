package sharedref

import (
	"reflect"
	"sync"
)

// SharedRef is a shared reference; copies of a SharedRef always refer
// to the same value, so a modification to any copy implies a state
// mutation across all copies.
type SharedRef[T any] struct {
	value **T
}

// New() creates a new SharedRef.
func New[T any](value T) SharedRef[T] {
	valueRef := &value

	instance := SharedRef[T]{
		value: &valueRef,
	}

	// Prevent pointers during runtime.
	rvalue := reflect.ValueOf(value)
	if rvalue.Kind() == reflect.Ptr {
		// Die.
		*instance.value = nil
	}

	return instance
}

// Dead() creates a dead SharedRef; it is useless, but replaces a nil
// value when we want to represent optionality using raw pointers.
func Dead[T any]() SharedRef[T] {
	var value T
	var pointer *T = nil

	instance := New(value)
	instance.value = &pointer

	return instance
}

func (this SharedRef[T]) Do(locker sync.Locker, body func(<-chan *T, chan<- *T)) {
	reader := make(chan *T)
	writer := make(chan *T)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		locker.Lock()
		body(reader, writer)
		wg.Done()
		locker.Unlock()
	}()

	reader <- *this.value
	close(reader)

	*this.value = <-writer
	close(writer)

	wg.Wait()
}

func (this SharedRef[T]) IsAlive() bool {
	return *this.value != nil
}

func (this SharedRef[T]) IsDead() bool {
	return *this.value == nil
}
