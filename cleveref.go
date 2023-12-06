package cleveref

import (
	"reflect"
)

type Immutable[T any] struct {
	// value is a *T as copying a pointer is cheaper than copying a
	// potentially large T.
	// Every time an Immutable is passed along the call stack, it
	// should be passed as a copy.  But we'll always be copying a
	// pointer, not an actual T.  This is why we hold a *T, not a T.
	value *T
}

func NewImmutable[T any](value T) Immutable[T] {
	kind := reflect.ValueOf(value).Kind()

	dead := Immutable[T]{value: nil}
	alive := Immutable[T]{value: &value}

	if kind == reflect.Ptr {
		return dead
	} else if kind == reflect.Map {
		return dead
	}

	return alive
}

func (i Immutable[T]) Use(continuation func(T)) {
	// When we dereference i.value, we get a T, and we pass this T to
	// the continuation (by value).
	// This is where a potentially expensive copy occurs, but it's the
	// way to guarantee we're not mutating the encapsulated value.
	if !i.IsDead() {
		continuation(*i.value)
	}
}

func (i Immutable[T]) IsDead() bool {
	return i.value == nil
}

func (i Immutable[T]) Swap(continuation func(T) T) Immutable[T] {
	if i.IsDead() {
		return i
	}

	newvalue := continuation(*i.value)
	return NewImmutable(newvalue)
}
