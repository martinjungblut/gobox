package box

import (
	"runtime"
	"sync"
	"testing"
)

// Counter is used by the test suite to observe state mutations.
type Counter struct {
	Value int
}

func (this *Counter) IncByReference() {
	this.Value++
}

func (this Counter) IncByValue() {
	this.Value++
}

func IncByAtomValue(atom Atom[Counter]) {
	atom.Use(func(counter *Counter) {
		counter.IncByReference()
	})
}

func IncByAtomReference(atom *Atom[Counter]) {
	atom.Use(func(counter *Counter) {
		counter.IncByReference()
	})
}

func Test_DoublePointer_Dies(t *testing.T) {
	a := 10
	b := &a
	c := &b
	atom := NewAtom(c)

	if !atom.IsDead() {
		t.Error("Atom should be dead.")
	}
}

func Test_Use_Swap_Alive(t *testing.T) {
	for a := 0; a < 10; a++ {
		called := false

		atom := NewAtom(&a)

		atom.Use(func(ptr *int) {
			called = true

			if *ptr != a {
				t.Errorf("Value should've been: %d", a)
			}
		})
		if !called {
			t.Error("Use() did not call its handler.")
		}

		called = false
		atom.Swap(func(ptr *int) *int {
			called = true

			if *ptr != a {
				t.Errorf("Value should've been: %d", a)
			}
			return ptr
		})
		if !called {
			t.Error("Swap() did not call its handler.")
		}
	}
}

func Test_Use_Swap_Dead(t *testing.T) {
	a := 0
	b := &a
	c := &b
	atom := NewAtom(c)

	called := false
	atom.Use(func(ptr **int) {
		called = true
	})
	if called {
		t.Error("Use() called its handler, even though the Atom was dead.")
	}

	called = false
	atom.Swap(func(ptr **int) **int {
		called = true
		return ptr
	})
	if called {
		t.Error("Swap() called its handler, even though the Atom was dead.")
	}
}

func Test_Mutation_Assumptions(t *testing.T) {
	// Observe some truths. IncByReference() should mutate,
	// IncByValue() should not.
	counter := Counter{Value: 0}

	counter.IncByReference()
	if counter.Value != 1 {
		t.Error("Method IncByReference() performed no mutation.")
	}

	counter.IncByValue()
	if counter.Value != 1 {
		t.Error("Method IncByValue() performed a mutation.")
	}
}

func Test_Mutation(t *testing.T) {
	counter := Counter{Value: 0}
	atom := NewAtom(&counter)

	// Call methods directly inside a Use() block. Regular Go
	// semantics apply.
	atom.Use(func(ptr *Counter) {
		// Value becomes 1.
		ptr.IncByReference()
		if ptr.Value != 1 {
			t.Error("Method IncByReference() performed no mutation.")
		}

		// Modifies the implicit copy, no mutation to the Counter
		// pointed to by 'ptr' is actually performed.
		ptr.IncByValue()
		if ptr.Value != 1 {
			t.Error("Method IncByValue() performed a mutation.")
		}
	})

	// Call methods inside another function that received the Atom as
	// a copy.
	// Value becomes 2.
	IncByAtomValue(atom)
	atom.Use(func(ptr *Counter) {
		if ptr.Value != 2 {
			t.Error("Function IncByAtomValue() performed no mutation.")
		}
	})

	// Call methods inside another function that received the Atom by
	// reference.
	// Value becomes 3.
	IncByAtomReference(&atom)
	atom.Use(func(ptr *Counter) {
		if ptr.Value != 3 {
			t.Error("Function IncByAtomReference() performed no mutation.")
		}
	})

	// Swap() directly on the 'atom' symbol. Mutates.
	// Value becomes 4.
	atom.Swap(func(ptr *Counter) *Counter {
		counter := *ptr
		counter.Value++
		return &counter
	})

	func(copy Atom[Counter]) {
		// Swap() on a local copy named 'copy'. Mutates.
		// Value becomes 5.
		copy.Swap(func(ptr *Counter) *Counter {
			counter := *ptr
			counter.Value++
			return &counter
		})
	}(atom)

	// We can see the original 'atom' was mutated here.
	atom.Use(func(ptr *Counter) {
		counter := *ptr
		if counter.Value != 5 {
			t.Error("Swap() performed no mutations.")
		}
	})
}

func Test_Swap_Nil_Pointer_Kills_Atom(t *testing.T) {
	a := 10
	atom := NewAtom(&a)

	atom.Swap(func(ptr *int) *int {
		return nil
	})

	if !atom.IsDead() {
		t.Error("Atom should be dead.")
	}
}

func Test_Atomicity(t *testing.T) {
	// We set MAXPROCS to NumCPU() + 1 as that ensures we're not
	// running in single-threaded mode. Therefore, even on a
	// single-core machine, forcing our runtime to use 2 OS threads
	// ensures the atomicity is actually verified.
	maxprocs := runtime.NumCPU() + 1
	runtime.GOMAXPROCS(maxprocs)

	cycles := 1000000
	// 'expectedValue' is cycles * 2 because we're checking both Use()
	// and Swap() during each iteration, so each iteration adds 2, not
	// 1.
	expectedValue := cycles * 2

	i := 0
	atom := NewAtom(&i)

	var wg sync.WaitGroup
	for i := 1; i <= cycles; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			atom.Use(func(ptr *int) {
				*ptr++
			})

			atom.Swap(func(ptr *int) *int {
				y := *ptr
				y++
				return &y
			})
		}()
	}
	wg.Wait()

	atom.Use(func(ptr *int) {
		if *ptr != expectedValue {
			t.Errorf("Atom is not atomic. Final value was %d, should have been %d.", *ptr, expectedValue)
		}
	})
}
