package atom

import (
	"reflect"
	"sync"
)

// Atom is a shared, atomic reference; copies of an Atom always refer
// to the same value, so a modification to any copy implies a state
// mutation across all copies.
type Atom[T any] struct {
	mutex        *sync.Mutex
	lockedByUse  *bool
	lockedBySwap *bool

	// 'value' must be a **T so that we can modify it from Atom
	// copies.
	value **T
}

func Dead[T any]() Atom[T] {
	var value *T = nil
	return Atom[T]{value: &value}
}

func New[T any](value T) Atom[T] {
	mutex := sync.Mutex{}
	lockedByUse := false
	lockedBySwap := false

	valueRef := &value
	instance := Atom[T]{
		mutex:        &mutex,
		lockedByUse:  &lockedByUse,
		lockedBySwap: &lockedBySwap,
		value:        &valueRef,
	}

	// Prevent pointers during runtime.
	rvalue := reflect.ValueOf(value)
	if rvalue.Kind() == reflect.Ptr {
		// Die.
		*instance.value = nil
	}

	return instance
}

// Use() takes a 'handler' func(*T) as its input, and passes the
// atom's value to said function, which is invoked atomically.
func (this Atom[T]) Use(handler func(*T)) bool {
	// Use() should never call 'handler' if the atom is dead.
	if this.IsDead() || *this.lockedBySwap {
		return false
	} else {
		this.mutex.Lock()
		*this.lockedByUse = true
		defer func() {
			*this.lockedByUse = false
			this.mutex.Unlock()
		}()

		handler(*this.value)
		return true
	}
}

// Swap() takes a 'handler' func(*T) *T as its input, and passes the
// atom's value to said function, which is invoked atomically; the
// value returned by this 'handler' is used as the atom's new value.
func (this Atom[T]) Swap(handler func(*T) *T) bool {
	// Swap() should never call its 'handler' if the atom is either
	// dead or locked.
	if this.IsDead() || this.IsLocked() {
		return false
	} else {
		this.mutex.Lock()
		*this.lockedBySwap = true
		defer func() {
			*this.lockedBySwap = false
			this.mutex.Unlock()
		}()

		// If 'newValue' is nil, the Atom will die.
		newValue := handler(*this.value)
		*this.value = newValue
		return true
	}
}

// IsDead() returns true if the atom is dead, meaning it cannot be
// used anymore.
func (this Atom[T]) IsDead() bool {
	return *this.value == nil
}

// IsLocked() returns true if the atom is currently locked by a call
// to Use() or Swap().
func (this Atom[T]) IsLocked() bool {
	return *this.lockedByUse || *this.lockedBySwap
}

// Nest() creates a new execution context for the atom; it is useful
// when two or more calls to Use() must be nested together; without
// calling Nest() to retrieve a new atom, nesting Use() will always
// deadlock.
func (this Atom[T]) Nest() Atom[T] {
	lockedByUse := false
	this.lockedByUse = &lockedByUse

	mutex := sync.Mutex{}
	this.mutex = &mutex

	return this
}

// SliceExtract() converts a slice of Atom[T] into a slice of T.
func SliceExtract[T any](input []Atom[T]) []T {
	output := make([]T, 0)

	for _, atom := range input {
		atom.Use(func(ptr *T) {
			output = append(output, *ptr)
		})
	}

	return output
}
