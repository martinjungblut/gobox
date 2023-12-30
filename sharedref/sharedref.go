package sharedref

import (
	"reflect"
	"sync"
	"time"
)

// SharedRef is a shared reference; copies of a SharedRef always refer
// to the same value, so a modification to any copy implies a state
// mutation across all copies.
type SharedRef[T any] struct {
	mutex           *sync.Mutex
	lockedByLocking *bool
	lockedBySwap    *bool

	// useCounter specifies how many Use() blocks are currently
	// running; it is guarded by a mutex
	useCounter      *int
	useCounterMutex *sync.Mutex

	contentionHandlers []func(time.Duration, func())

	// 'value' must be a **T so that we can modify it from SharedRef
	// copies.
	value **T
}

// Dead() creates a dead SharedRef; it is useless, but replaces a nil
// value when we want to represent optionality using raw pointers.
func Dead[T any]() SharedRef[T] {
	var value T

	instance := New(value)
	instance.Swap(func(_ *T) *T {
		return nil
	})

	return instance
}

// New() creates a new SharedRef.
func New[T any](value T, contentionHandlers ...func(time.Duration, func())) SharedRef[T] {
	mutex := sync.Mutex{}
	lockedByLocking := false
	lockedBySwap := false

	useCounter := 0
	useCounterMutex := sync.Mutex{}

	valueRef := &value
	instance := SharedRef[T]{
		mutex:              &mutex,
		lockedByLocking:    &lockedByLocking,
		lockedBySwap:       &lockedBySwap,
		useCounter:         &useCounter,
		useCounterMutex:    &useCounterMutex,
		contentionHandlers: contentionHandlers,
		value:              &valueRef,
	}

	// Prevent pointers during runtime.
	rvalue := reflect.ValueOf(value)
	if rvalue.Kind() == reflect.Ptr {
		// Die.
		*instance.value = nil
	}

	return instance
}

// Use() takes a 'handler' function as its input, and passes the
// SharedRef's value to said function.
func (this SharedRef[T]) Use(handler func(*T)) bool {
	// Use() should never call 'handler' if the SharedRef is dead or
	// locked by Swap().
	if this.IsDead() || *this.lockedBySwap {
		return false
	} else {
		this.useCounterMutex.Lock()
		*this.useCounter++
		this.useCounterMutex.Unlock()

		handler(*this.value)

		this.useCounterMutex.Lock()
		*this.useCounter--
		this.useCounterMutex.Unlock()

		return true
	}
}

// Locking() takes a 'handler' function as its input, and calls this
// handler function atomically.
func (this SharedRef[T]) Locking(handler func()) bool {
	if this.IsDead() || *this.lockedBySwap {
		// Locking() should never call 'handler' if the SharedRef is
		// dead or locked by Swap().
		return false
	} else if !this.tryLock(this.mutex) {
		// If tryLock() gives up due to contention, give up.
		return false
	}

	*this.lockedByLocking = true
	defer func() {
		*this.lockedByLocking = false
		this.mutex.Unlock()
	}()

	handler()
	return true
}

// Swap() takes a 'handler' function as its input, and passes the
// SharedRef's pointer value to said function, which is invoked
// atomically; the pointer value returned by this 'handler' is used as
// the SharedRef's new value.
func (this SharedRef[T]) Swap(handler func(*T) *T) bool {
	if this.IsDead() || *this.lockedByLocking || *this.useCounter > 0 {
		// Swap() should never call its 'handler' if the SharedRef is
		// dead, inside a Locking() block, or inside a Use() block.
		return false
	} else if !this.tryLock(this.mutex) {
		// If tryLock() gives up due to contention, give up.
		return false
	}

	*this.lockedBySwap = true
	defer func() {
		*this.lockedBySwap = false
		this.mutex.Unlock()
	}()

	// If 'newValue' is nil, the SharedRef will die.
	newValue := handler(*this.value)
	*this.value = newValue
	return true
}

// IsDead() returns true if the SharedRef is dead, meaning it cannot
// be used anymore.
func (this SharedRef[T]) IsDead() bool {
	return *this.value == nil
}

// IsAlive() returns true if the SharedRef is alive, meaning it can be used
// normally.
func (this SharedRef[T]) IsAlive() bool {
	return *this.value != nil
}

// IsLocked() returns true if the SharedRef is currently locked by a call
// to Use() or Swap().
func (this SharedRef[T]) IsLocked() bool {
	return *this.lockedByLocking || *this.lockedBySwap
}

func (this SharedRef[T]) tryLock(mutex *sync.Mutex) bool {
	attempts := 0
	gaveUp := false
	giveUp := func() {
		gaveUp = true
	}

	timeBefore := time.Now()

	for !mutex.TryLock() {
		attempts++
		time.Sleep(time.Microsecond * 1)
		timeElapsed := time.Now().Sub(timeBefore)

		for index := range this.contentionHandlers {
			callable := this.contentionHandlers[index]
			callable(timeElapsed, giveUp)
		}

		if gaveUp {
			return false
		}
	}

	return true
}

// SliceExtract() converts a slice of SharedRef[T] into a slice of T.
func SliceExtract[T any](input []SharedRef[T]) []T {
	output := make([]T, 0)

	for _, ref := range input {
		ref.Use(func(ptr *T) {
			output = append(output, *ptr)
		})
	}

	return output
}
