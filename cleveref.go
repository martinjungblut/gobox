package cleveref

import (
	"reflect"
)

type CleverefPanic int

const (
	// default value for int is 0, so PANIC_NONE means no panic
	PANIC_NONE CleverefPanic = iota

	PANIC_UNEXPECTED_POINTER CleverefPanic = iota
	PANIC_UNEXPECTED_MAP     CleverefPanic = iota
)

func Recover(r any, continuation func(CleverefPanic)) {
	if p, ok := r.(CleverefPanic); ok && p != PANIC_NONE {
		continuation(p)
	}
}

type Immutable[T any] struct {
	// value is a *T as copying a pointer is cheaper than copying a
	// potentially large T.
	value *T
}

func NewImmutable[T any](value T) Immutable[T] {
	kind := reflect.ValueOf(value).Kind()
	if kind == reflect.Ptr {
		panic(PANIC_UNEXPECTED_POINTER)
	} else if kind == reflect.Map {
		panic(PANIC_UNEXPECTED_MAP)
	}

	return Immutable[T]{value: &value}
}

func (i Immutable[T]) Use(continuation func(T)) {
	// When we dereference i.value, we get a T, and we pass this T to
	// the continuation (by value).
	// This is where a potentially expensive copy occurs.
	continuation(*i.value)
}
