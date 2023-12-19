package atom

import (
	"reflect"
	"sync"
)

// Atom is a shared, atomic reference. Copies of an Atom always refer
// to the same Atom, so a modification to any copy implies a state
// mutation across all copies.
type Atom[T any] struct {
	mutex        *sync.Mutex
	lockedByUse  *bool
	lockedBySwap *bool

	// 'value' must be a **T so that we can modify it from Atom
	// copies.
	value **T
}

// Type *T enforces a pointer must be used, during compile-time.
func NewAtom[T any](value *T) Atom[T] {
	mutex := sync.Mutex{}
	lockedByUse := false
	lockedBySwap := false

	instance := Atom[T]{
		mutex:        &mutex,
		lockedByUse:  &lockedByUse,
		lockedBySwap: &lockedBySwap,
		value:        &value,
	}

	// Prevent double pointers during runtime.
	rvalue := reflect.ValueOf(value)
	if rvalue.Kind() == reflect.Ptr && rvalue.Elem().Kind() == reflect.Ptr {
		// Die.
		*instance.value = nil
	}

	return instance
}

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

func (this Atom[T]) IsDead() bool {
	return *this.value == nil
}

func (this Atom[T]) IsLocked() bool {
	return *this.lockedByUse || *this.lockedBySwap
}

func (this Atom[T]) Nest() Atom[T] {
	lockedByUse := false
	this.lockedByUse = &lockedByUse

	mutex := sync.Mutex{}
	this.mutex = &mutex

	return this
}
