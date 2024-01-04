package atom

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func Concurrently(times int, handler func()) {
	maxprocs := runtime.NumCPU() + 1
	runtime.GOMAXPROCS(maxprocs)

	wg := sync.WaitGroup{}
	wg.Add(times)
	for i := 1; i <= times; i++ {
		go func() {
			defer wg.Done()

			handler()
		}()
	}
	wg.Wait()
}

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

func IncByValue(atom Atom[Counter]) {
	atom.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		counter := <-portal.Reader
		counter.IncByReference()
		portal.Writer <- counter
	})
}

func IncByReference(atom *Atom[Counter]) {
	atom.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		counter := <-portal.Reader
		counter.IncByReference()
		portal.Writer <- counter
	})
}

func Test_IsAlive(t *testing.T) {
	atom := New(0)

	if !atom.IsAlive() {
		t.Error("Should be alive.")
	}
}

func Test_IsDead(t *testing.T) {
	atom := Dead[int]()

	if !atom.IsDead() {
		t.Error("Should be dead.")
	}
}

func Test_Pointer_Kills(t *testing.T) {
	number := 10
	atom := New(&number)

	if !atom.IsDead() {
		t.Error("Should be dead.")
	}
}

func Test_NoCopy(t *testing.T) {
	atom := New(atomic.Bool{})
	check := false

	atom.Do(&sync.Mutex{}, func(portal Portal[atomic.Bool]) {
		boolean := <-portal.Reader
		boolean.Store(true)

		portal.Writer <- boolean
		check = true
	})

	if !check {
		t.Error("Check failed.")
	}
}

func Test_Do_Dead(t *testing.T) {
	atom := Dead[int]()

	called := false

	atom.Do(&sync.Mutex{}, func(portal Portal[int]) {
		pointer := <-portal.Reader
		portal.Writer <- pointer
		called = true
	})

	if called {
		t.Error("Do() should not invoke its body if the Atom is dead.")
	}
}

func Test_Do_Atomicity(t *testing.T) {
	atom := New(0)
	cycles := 1000
	mutex := &sync.Mutex{}

	Concurrently(cycles, func() {
		atom.Do(mutex, func(portal Portal[int]) {
			pointer := <-portal.Reader

			value := *pointer
			value++

			portal.Writer <- &value
		})
	})

	atom.Do(mutex, func(portal Portal[int]) {
		pointer := <-portal.Reader
		value := *pointer

		if value != cycles {
			t.Errorf("value was '%d', but should have been '%d'.", value, cycles)
		}

		portal.Writer <- pointer
	})
}

func Test_Do_Nesting(t *testing.T) {
	atom := New(0)

	check1, check2, check3 := false, false, false
	mutexA := &sync.Mutex{}
	mutexB := &sync.Mutex{}

	atom.Do(mutexA, func(portalA Portal[int]) {
		pointerA := <-portalA.Reader
		*pointerA++

		atom.Do(mutexB, func(portalB Portal[int]) {
			pointerB := <-portalB.Reader
			*pointerB++

			check2 = true
			portalB.Writer <- pointerB
		})

		check1 = true
		portalA.Writer <- pointerA
	})

	atom.Do(mutexA, func(portal Portal[int]) {
		pointer := <-portal.Reader
		portal.Writer <- pointer

		if *pointer != 2 {
			t.Errorf("Value should be 2, but instead it was: '%d'.", *pointer)
		}

		// Ensure method runs until the end, and that this actually
		// behaves synchronously.
		check3 = true
	})

	if !check1 {
		t.Error("Check 1 failed.")
	}

	if !check2 {
		t.Error("Check 2 failed.")
	}

	if !check3 {
		t.Error("Check 3 failed. Code might have executed asynchronously.")
	}
}

func Test_Do_Reader_Writer(t *testing.T) {
	atom := New(0)
	mutex := &sync.Mutex{}

	atom.Do(mutex, func(portal Portal[int]) {
		pointer := <-portal.Reader
		if pointer == nil {
			t.Error("Reading the first time should not be nil.")
		}
		if <-portal.Reader != nil {
			t.Error("Reading a second time should be nil.")
		}

		portal.Writer <- pointer
		// This would panic, as the writer has already been closed.
		// portal.Writer <- pointer
	})

	atom.Do(mutex, func(portal Portal[int]) {
		pointer := <-portal.Reader
		if pointer == nil {
			t.Error("Reading the should not be nil.")
		}
		portal.Writer <- pointer
	})
}

func Test_Do_Last_Write_Wins(t *testing.T) {
	atom := New(0)

	mutexA := &sync.Mutex{}
	mutexB := &sync.Mutex{}

	atom.Do(mutexA, func(portalA Portal[int]) {
		atom.Do(mutexB, func(portalB Portal[int]) {
			pointerB := <-portalB.Reader
			*pointerB++
			portalB.Writer <- pointerB
		})

		pointerA := <-portalA.Reader
		if *pointerA != 1 {
			t.Errorf("Value should be 1, but instead it was: '%d'.", *pointerA)
		}
		portalA.Writer <- nil
	})

	if !atom.IsDead() {
		t.Error("Should be dead.")
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
	atom := New(Counter{Value: 0})

	// Call methods directly inside a Use() block. Regular Go
	// semantics apply.
	atom.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		pointer := <-portal.Reader

		// Value becomes 1.
		pointer.IncByReference()
		if pointer.Value != 1 {
			t.Error("Method IncByReference() performed no mutation.")
		}

		// Modifies the implicit copy, no mutation to the Counter
		// pointed to by 'pointer' is actually performed.
		pointer.IncByValue()
		if pointer.Value != 1 {
			t.Error("Method IncByValue() performed a mutation.")
		}

		portal.Writer <- pointer
	})

	// Call methods inside another function that received the
	// Atom as a copy.
	// Value becomes 2.
	IncByValue(atom)
	atom.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 2 {
			t.Error("Function IncByValue() performed no mutation.")
		}
		portal.Writer <- pointer
	})

	// Call methods inside another function that received the
	// Atom by reference.
	// Value becomes 3.
	IncByReference(&atom)
	atom.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 3 {
			t.Error("Function IncByReference() performed no mutation.")
		}
		portal.Writer <- pointer
	})

	func(copy Atom[Counter]) {
		// Do() on a local copy named 'copy'. Mutates.
		// Value becomes 4.
		copy.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
			pointer := <-portal.Reader
			pointer.Value++
			portal.Writer <- pointer
		})
	}(atom)

	// We can see the original 'atom' was mutated here.
	atom.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 4 {
			t.Error("Do() performed no mutations.")
		}
		portal.Writer <- pointer
	})
}
