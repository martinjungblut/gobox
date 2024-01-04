package sharedref

import (
	"runtime"
	"sync"
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
	sharedref.Do(&sync.Mutex{}, func(reader <-chan *Counter, writer chan<- *Counter) {
		counter := <-reader
		counter.IncByReference()
		writer <- counter
	})
}

func IncByReference(sharedref *SharedRef[Counter]) {
	sharedref.Do(&sync.Mutex{}, func(reader <-chan *Counter, writer chan<- *Counter) {
		counter := <-reader
		counter.IncByReference()
		writer <- counter
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

func Test_Atomicity(t *testing.T) {
	sharedref := New(0)
	cycles := 1000
	mutex := &sync.Mutex{}

	Concurrently(cycles, func() {
		sharedref.Do(mutex, func(reader <-chan *int, writer chan<- *int) {
			ptr := <-reader

			value := *ptr
			value++

			writer <- &value
		})
	})

	sharedref.Do(mutex, func(reader <-chan *int, writer chan<- *int) {
		ptr := <-reader
		value := *ptr

		if value != cycles {
			t.Errorf("value was '%d', but should have been '%d'.", value, cycles)
		}

		writer <- ptr
	})
}

func Test_Nesting(t *testing.T) {
	sharedref := New(0)

	check1, check2, check3 := false, false, false
	mutexA := &sync.Mutex{}
	mutexB := &sync.Mutex{}

	sharedref.Do(mutexA, func(reader <-chan *int, writer chan<- *int) {
		pointerA := <-reader
		*pointerA++

		sharedref.Do(mutexB, func(readerB <-chan *int, writerB chan<- *int) {
			pointerB := <-readerB
			*pointerB++

			check2 = true
			writerB <- pointerB
		})

		check1 = true
		writer <- pointerA
	})

	sharedref.Do(mutexA, func(reader <-chan *int, writer chan<- *int) {
		ptr := <-reader
		writer <- ptr

		if *ptr != 2 {
			t.Errorf("Value should be 2, but instead it was: '%d'.", *ptr)
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

func Test_Reader_Writer(t *testing.T) {
	sharedref := New(0)
	mutex := &sync.Mutex{}

	sharedref.Do(mutex, func(reader <-chan *int, writer chan<- *int) {
		pointer := <-reader
		if pointer == nil {
			t.Error("Reading the first time should not be nil.")
		}
		if <-reader != nil {
			t.Error("Reading a second time should be nil.")
		}

		writer <- pointer
		// This would panic, as the writer has already been closed.
		// writer <- pointer
	})

	sharedref.Do(mutex, func(reader <-chan *int, writer chan<- *int) {
		pointer := <-reader
		if pointer == nil {
			t.Error("Reading the should not be nil.")
		}
		writer <- pointer
	})
}

func Test_Last_Write_Wins(t *testing.T) {
	sharedref := New(0)

	mutexA := &sync.Mutex{}
	mutexB := &sync.Mutex{}

	sharedref.Do(mutexA, func(readerA <-chan *int, writerA chan<- *int) {
		sharedref.Do(mutexB, func(readerB <-chan *int, writerB chan<- *int) {
			pointerB := <-readerB
			*pointerB++
			writerB <- pointerB
		})

		pointerA := <-readerA
		if *pointerA != 1 {
			t.Errorf("Value should be 1, but instead it was: '%d'.", *pointerA)
		}
		writerA <- nil
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
	sharedref.Do(&sync.Mutex{}, func(reader <-chan *Counter, writer chan<- *Counter) {
		pointer := <-reader

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

		writer <- pointer
	})

	// Call methods inside another function that received the
	// SharedRef as a copy.
	// Value becomes 2.
	IncByValue(sharedref)
	sharedref.Do(&sync.Mutex{}, func(reader <-chan *Counter, writer chan<- *Counter) {
		pointer := <-reader
		if pointer.Value != 2 {
			t.Error("Function IncByValue() performed no mutation.")
		}
		writer <- pointer
	})

	// Call methods inside another function that received the
	// SharedRef by reference.
	// Value becomes 3.
	IncByReference(&sharedref)
	sharedref.Do(&sync.Mutex{}, func(reader <-chan *Counter, writer chan<- *Counter) {
		pointer := <-reader
		if pointer.Value != 3 {
			t.Error("Function IncByReference() performed no mutation.")
		}
		writer <- pointer
	})

	func(copy SharedRef[Counter]) {
		// Do() on a local copy named 'copy'. Mutates.
		// Value becomes 4.
		copy.Do(&sync.Mutex{}, func(reader <-chan *Counter, writer chan<- *Counter) {
			pointer := <-reader
			pointer.Value++
			writer <- pointer
		})
	}(sharedref)

	// We can see the original 'sharedref' was mutated here.
	sharedref.Do(&sync.Mutex{}, func(reader <-chan *Counter, writer chan<- *Counter) {
		pointer := <-reader
		if pointer.Value != 4 {
			t.Error("Do() performed no mutations.")
		}
		writer <- pointer
	})
}
