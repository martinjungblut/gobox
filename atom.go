package box

import (
	"reflect"
	"sync"
)

// Atom is a shared, atomic reference. Copies of an Atom always refer
// to the same Atom, so a modification to any copy implies a state
// mutation across all copies.
type Atom[T any] struct {
	mutex *sync.Mutex
	// 'value' must be a **T so that we can modify it from Atom
	// copies.
	value **T
}

// Type *T enforces a pointer must be used, during compile-time.
func NewAtom[T any](value *T) Atom[T] {
	mutex := sync.Mutex{}

	instance := Atom[T]{value: &value, mutex: &mutex}

	// Prevent double pointers during runtime.
	rvalue := reflect.ValueOf(value)
	if rvalue.Kind() == reflect.Ptr && rvalue.Elem().Kind() == reflect.Ptr {
		// Die.
		*instance.value = nil
	}

	return instance
}

func (this Atom[T]) Use(handler func(*T)) {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	if !this.IsDead() {
		handler(*this.value)
	}
}

func (this Atom[T]) Swap(handler func(*T) *T) {
	this.mutex.Lock()
	defer this.mutex.Unlock()

	if !this.IsDead() {
		// If 'newValue' is nil, the Atom will die.
		newValue := handler(*this.value)
		*this.value = newValue
	}
}

func (this Atom[T]) IsDead() bool {
	return *this.value == nil
}
