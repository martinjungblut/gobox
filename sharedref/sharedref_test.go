package sharedref

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

func IncByValue(sharedref SharedRef[Counter]) {
	sharedref.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		counter := <-portal.Reader
		counter.IncByReference()
		portal.Writer <- counter
	})
}

func IncByReference(sharedref *SharedRef[Counter]) {
	sharedref.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		counter := <-portal.Reader
		counter.IncByReference()
		portal.Writer <- counter
	})
}

func Test_IsAlive(t *testing.T) {
	sharedref := New(0)

	if !sharedref.IsAlive() {
		t.Error("Should be alive.")
	}
}

func Test_IsDead(t *testing.T) {
	sharedref := Dead[int]()

	if !sharedref.IsDead() {
		t.Error("Should be dead.")
	}
}

func Test_Pointer_Kills(t *testing.T) {
	number := 10
	sharedref := New(&number)

	if !sharedref.IsDead() {
		t.Error("Should be dead.")
	}
}

func Test_NoCopy(t *testing.T) {
	sharedref := New(atomic.Bool{})
	check := false

	sharedref.Do(&sync.Mutex{}, func(portal Portal[atomic.Bool]) {
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
	sharedref := Dead[int]()

	called := false

	sharedref.Do(&sync.Mutex{}, func(portal Portal[int]) {
		pointer := <-portal.Reader
		portal.Writer <- pointer
		called = true
	})

	if called {
		t.Error("Do() should not invoke its body if the SharedRef is dead.")
	}
}

func Test_Do_Atomicity(t *testing.T) {
	sharedref := New(0)
	cycles := 1000
	mutex := &sync.Mutex{}

	Concurrently(cycles, func() {
		sharedref.Do(mutex, func(portal Portal[int]) {
			pointer := <-portal.Reader

			value := *pointer
			value++

			portal.Writer <- &value
		})
	})

	sharedref.Do(mutex, func(portal Portal[int]) {
		pointer := <-portal.Reader
		value := *pointer

		if value != cycles {
			t.Errorf("value was '%d', but should have been '%d'.", value, cycles)
		}

		portal.Writer <- pointer
	})
}

func Test_Do_Nesting(t *testing.T) {
	sharedref := New(0)

	check1, check2, check3 := false, false, false
	mutexA := &sync.Mutex{}
	mutexB := &sync.Mutex{}

	sharedref.Do(mutexA, func(portalA Portal[int]) {
		pointerA := <-portalA.Reader
		*pointerA++

		sharedref.Do(mutexB, func(portalB Portal[int]) {
			pointerB := <-portalB.Reader
			*pointerB++

			check2 = true
			portalB.Writer <- pointerB
		})

		check1 = true
		portalA.Writer <- pointerA
	})

	sharedref.Do(mutexA, func(portal Portal[int]) {
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
	sharedref := New(0)
	mutex := &sync.Mutex{}

	sharedref.Do(mutex, func(portal Portal[int]) {
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

	sharedref.Do(mutex, func(portal Portal[int]) {
		pointer := <-portal.Reader
		if pointer == nil {
			t.Error("Reading the should not be nil.")
		}
		portal.Writer <- pointer
	})
}

func Test_Do_Last_Write_Wins(t *testing.T) {
	sharedref := New(0)

	mutexA := &sync.Mutex{}
	mutexB := &sync.Mutex{}

	sharedref.Do(mutexA, func(portalA Portal[int]) {
		sharedref.Do(mutexB, func(portalB Portal[int]) {
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

	if !sharedref.IsDead() {
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
	sharedref := New(Counter{Value: 0})

	// Call methods directly inside a Use() block. Regular Go
	// semantics apply.
	sharedref.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
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
	// SharedRef as a copy.
	// Value becomes 2.
	IncByValue(sharedref)
	sharedref.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 2 {
			t.Error("Function IncByValue() performed no mutation.")
		}
		portal.Writer <- pointer
	})

	// Call methods inside another function that received the
	// SharedRef by reference.
	// Value becomes 3.
	IncByReference(&sharedref)
	sharedref.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 3 {
			t.Error("Function IncByReference() performed no mutation.")
		}
		portal.Writer <- pointer
	})

	func(copy SharedRef[Counter]) {
		// Do() on a local copy named 'copy'. Mutates.
		// Value becomes 4.
		copy.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
			pointer := <-portal.Reader
			pointer.Value++
			portal.Writer <- pointer
		})
	}(sharedref)

	// We can see the original 'sharedref' was mutated here.
	sharedref.Do(&sync.Mutex{}, func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 4 {
			t.Error("Do() performed no mutations.")
		}
		portal.Writer <- pointer
	})
}
