package atom

import (
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
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

func Test_Dead_IsDead_IsAlive(t *testing.T) {
	atom := Dead[int]()

	if !atom.IsDead() {
		t.Error("Atom should be dead.")
	}

	if atom.IsAlive() {
		t.Error("Atom should be dead.")
	}
}

func Test_Pointer_Dies(t *testing.T) {
	a := 10
	atom := New(&a)

	if !atom.IsDead() {
		t.Error("Atom should be dead.")
	}
}

func Test_Use_And_Swap_Alive(t *testing.T) {
	for a := 0; a < 10; a++ {
		atom := New(a)

		called := false
		result := atom.Use(func(ptr *int) {
			called = true

			if *ptr != a {
				t.Errorf("Value should've been: %d", a)
			}
		})
		if !called {
			t.Error("Use() did not call its handler.")
		}
		if !result {
			t.Error("Use() did not return true.")
		}

		called = false
		result = atom.Swap(func(ptr *int) *int {
			called = true

			if *ptr != a {
				t.Errorf("Value should've been: %d", a)
			}
			return ptr
		})
		if !called {
			t.Error("Swap() did not call its handler.")
		}
		if !result {
			t.Error("Swap() did not return true.")
		}
	}
}

func Test_Use_And_Swap_Dead(t *testing.T) {
	a := 0
	atom := New(&a)

	called := false
	result := atom.Use(func(ptr **int) {
		called = true
	})
	if called {
		t.Error("Use() called its handler, even though the Atom was dead.")
	}
	if result {
		t.Error("Use() did not return false.")
	}

	called = false
	result = atom.Swap(func(ptr **int) **int {
		called = true
		return ptr
	})
	if called {
		t.Error("Swap() called its handler, even though the Atom was dead.")
	}
	if result {
		t.Error("Swap() did not return false.")
	}
}

func Test_Swap_Nil_Pointer_Kills_Atom(t *testing.T) {
	atom := New(10)

	atom.Swap(func(ptr *int) *int {
		return nil
	})

	if !atom.IsDead() {
		t.Error("Atom should be dead.")
	}
}

func Test_Mutation_Assumptions(t *testing.T) {
	// Observe some truths. IncByReference() should mutate,
	// IncByValue() should not. These are truths are implied by the
	// semantics of Go, but the test simply makes them explicitly
	// verifiable.
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
	atom := New(counter)

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

func Test_Atomicity_And_Nested_And_IsLocked(t *testing.T) {
	// We set MAXPROCS to NumCPU() + 1 as that ensures we're not
	// running in single-threaded mode. Therefore, even on a
	// single-core machine, forcing our runtime to use 2 OS threads
	// ensures the atomicity is actually verified.
	maxprocs := runtime.NumCPU() + 1
	runtime.GOMAXPROCS(maxprocs)

	cycles := 1000000
	expectedValue := 0
	expectedValueLock := sync.Mutex{}
	wg := sync.WaitGroup{}

	atom := New(0)

	for i := 1; i <= cycles; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			successful := atom.Use(func(ptr *int) {
				*ptr++

				// Ensure Atom is always locked when inside Use() handler.
				if !atom.IsLocked() {
					t.Error("Atom should be locked when inside Use() handler.")
				}

				nested := atom.Nest()
				successful := nested.Use(func(nptr *int) {
					*nptr++

					// Ensure nested Atom is always locked when inside Use() handler.
					if !nested.IsLocked() {
						t.Error("Nested atom should be locked when inside Use() handler.")
					}
				})
				if successful {
					expectedValueLock.Lock()
					expectedValue++
					expectedValueLock.Unlock()
				}
			})
			if successful {
				expectedValueLock.Lock()
				expectedValue++
				expectedValueLock.Unlock()
			}

			successful = atom.Swap(func(ptr *int) *int {
				y := *ptr
				y++

				// Ensure Atom is always locked when inside Swap() handler.
				if !atom.IsLocked() {
					t.Error("Atom should be locked when inside Swap() handler.")
				}

				return &y
			})
			if successful {
				expectedValueLock.Lock()
				expectedValue++
				expectedValueLock.Unlock()
			}
		}()
	}
	wg.Wait()

	atom.Use(func(ptr *int) {
		if *ptr != expectedValue {
			t.Errorf("Atom is not atomic. Final value was %d, should have been %d.", *ptr, expectedValue)
		}
	})
}

func Test_Nesting_And_Deadlocks(t *testing.T) {
	atom := New(0)

	resultFirstUse := false
	resultSecondUse := false

	resultSwap := false

	go func() {
		time.Sleep(time.Second * 3)
		deadlocked := false

		if !(resultFirstUse && resultSecondUse) {
			deadlocked = true
			t.Error("Deadlock detected when calling Use().")
		}

		if !(resultSwap) {
			deadlocked = true
			t.Error("Deadlock detected when calling Swap().")
		}

		if deadlocked {
			os.Exit(1)
		}
	}()

	resultFirstUse = atom.Use(func(p1 *int) {
		*p1++

		resultSecondUse = atom.Nest().Use(func(p2 *int) {
			*p2++
		})
	})

	if !atom.Use(func(ptr *int) {
		if *ptr != 2 {
			t.Error("Value should have been incremented 2 times.")
		}
	}) {
		t.Error("Atom is dead.")
	}

	resultSwap = atom.Swap(func(p1 *int) *int {
		y := *p1
		y++

		// Use() must never call its handler inside another Swap().
		resultNested := atom.Use(func(_ *int) {
		})

		// Swap() must never call its handler inside another Swap().
		resultNested = resultNested || atom.Swap(func(_ *int) *int {
			return nil
		})

		// Swap() from a nested Atom must never call its handler
		// inside another Swap().
		resultNested = resultNested || atom.Nest().Swap(func(_ *int) *int {
			return nil
		})

		if resultNested || atom.IsDead() {
			t.Error("Nested Use() or Swap() handler was called.")
		}

		return &y
	})

	if !atom.Use(func(ptr *int) {
		if *ptr != 3 {
			t.Error("Value should have been incremented 3 times.")
		}
	}) {
		t.Error("Atom is dead.")
	}
}

func Test_SliceExtract(t *testing.T) {
	input := []Atom[int]{New(1), New(2), New(3)}
	output := SliceExtract(input)

	if len(output) != 3 {
		t.Error("Slice length should be 3.")
	}

	if output[0] != 1 {
		t.Error("Unexpected value.")
	}

	if output[1] != 2 {
		t.Error("Unexpected value.")
	}

	if output[2] != 3 {
		t.Error("Unexpected value.")
	}
}
