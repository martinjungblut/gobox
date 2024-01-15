package sharef

import (
	"runtime"
	"sync"
	"testing"
)

func AssertPanic(body func(), message string, t *testing.T) {
	panicked := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		body()
	}()

	if !panicked {
		t.Fatal(message)
	}
}

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

func IncByValue(sharef Sharef[Counter]) {
	sharef.Do(func(portal Portal[Counter]) {
		counter := <-portal.Reader
		counter.IncByReference()
		portal.Writer <- counter
	})
}

func IncByReference(sharef *Sharef[Counter]) {
	sharef.Do(func(portal Portal[Counter]) {
		counter := <-portal.Reader
		counter.IncByReference()
		portal.Writer <- counter
	})
}

func Test_Sharef_New(t *testing.T) {
	New(0)
}

func Test_Sharef_New_Pointer_Panics(t *testing.T) {
	AssertPanic(func() {
		number := 10
		New(&number)
	}, "Pointer should have caused a panic.", t)
}

func Test_Sharef_Do_ZeroValue_Panics(t *testing.T) {
	AssertPanic(func() {
		var sharef Sharef[int]

		sharef.Do(func(portal Portal[int]) {
			ptr := <-portal.Reader
			portal.Writer <- ptr
		})
	}, "Zero value should have caused a panic.", t)
}

func Test_Sharef_Do_Nil_Panics(t *testing.T) {
	sharef := New(0)

	sharef.Do(func(portal Portal[int]) {
		<-portal.Reader
		portal.Writer <- nil
	})

	AssertPanic(func() {
		sharef.Do(func(portal Portal[int]) {
			ptr := <-portal.Reader
			portal.Writer <- ptr
		})
	}, "Nil value should have caused a panic.", t)
}

func Test_Sharef_Do_Atomicity(t *testing.T) {
	cycles := 100000

	sharef := New(0)
	mutex := &sync.Mutex{}

	Concurrently(cycles, func() {
		mutex.Lock()
		defer mutex.Unlock()

		sharef.Do(func(portal Portal[int]) {
			pointer := <-portal.Reader

			value := *pointer
			value++

			portal.Writer <- &value
		})
	})

	sharef.Do(func(portal Portal[int]) {
		pointer := <-portal.Reader
		value := *pointer

		if value != cycles {
			t.Fatalf("value was '%d', but should have been '%d'.", value, cycles)
		}

		portal.Writer <- pointer
	})
}

func Test_Sharef_Do_Nesting(t *testing.T) {
	sharef := New(0)

	check1, check2, check3 := false, false, false

	sharef.Do(func(portalA Portal[int]) {
		pointerA := <-portalA.Reader
		*pointerA++

		sharef.Do(func(portalB Portal[int]) {
			pointerB := <-portalB.Reader
			*pointerB++

			check2 = true
			portalB.Writer <- pointerB
		})

		check1 = true
		portalA.Writer <- pointerA
	})

	sharef.Do(func(portal Portal[int]) {
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

func Test_Sharef_Do_Reader_And_Writer_Are_Automatically_Closed(t *testing.T) {
	sharef := New(0)
	panicked := false

	sharef.Do(func(portal Portal[int]) {
		pointer := <-portal.Reader
		if pointer == nil {
			t.Error("Reading the first time should not be nil.")
		}
		if <-portal.Reader != nil {
			t.Error("Reading a second time should be nil.")
		}

		portal.Writer <- pointer

		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()

		// This should panic.
		portal.Writer <- pointer
	})

	sharef.Do(func(portal Portal[int]) {
		pointer := <-portal.Reader
		if pointer == nil {
			t.Error("Pointer should not be nil.")
		}
		portal.Writer <- pointer
	})

	if !panicked {
		t.Error("Second write should have caused a panic.")
	}
}

func Test_Sharef_Do_Last_Write_Wins(t *testing.T) {
	sharef := New(0)
	ten := 10

	sharef.Do(func(portalA Portal[int]) {
		sharef.Do(func(portalB Portal[int]) {
			pointerB := <-portalB.Reader
			*pointerB++
			portalB.Writer <- pointerB
		})

		pointerA := <-portalA.Reader
		if *pointerA != 1 {
			t.Errorf("Value should be 1, but instead it was: '%d'.", *pointerA)
		}
		portalA.Writer <- &ten
	})

	sharef.Do(func(portal Portal[int]) {
		pointer := <-portal.Reader

		if *pointer != 10 {
			t.Errorf("Value should be 10, but instead it was: '%d'.", *pointer)
		}

		portal.Writer <- pointer
	})
}

func Test_Sharef_Mutation_Assumptions(t *testing.T) {
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

func Test_Sharef_Mutation(t *testing.T) {
	sharef := New(Counter{Value: 0})

	// Call methods directly inside a Use() block. Regular Go
	// semantics apply.
	sharef.Do(func(portal Portal[Counter]) {
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

	// Call methods inside another function that received the Sharef
	// as a copy.
	// Value becomes 2.
	IncByValue(sharef)
	sharef.Do(func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 2 {
			t.Error("Function IncByValue() performed no mutation.")
		}
		portal.Writer <- pointer
	})

	// Call methods inside another function that received the Sharef
	// by reference.
	// Value becomes 3.
	IncByReference(&sharef)
	sharef.Do(func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 3 {
			t.Error("Function IncByReference() performed no mutation.")
		}
		portal.Writer <- pointer
	})

	func(copy Sharef[Counter]) {
		// Do() on a local copy named 'copy'. Mutates.
		// Value becomes 4.
		copy.Do(func(portal Portal[Counter]) {
			pointer := <-portal.Reader
			pointer.Value++
			portal.Writer <- pointer
		})
	}(sharef)

	// We can see the original 'sharef' was mutated here.
	sharef.Do(func(portal Portal[Counter]) {
		pointer := <-portal.Reader
		if pointer.Value != 4 {
			t.Error("Do() performed no mutations.")
		}
		portal.Writer <- pointer
	})
}

func Test_Group_New_Pointer_Panics(t *testing.T) {
	AssertPanic(func() {
		x := 10

		group := NewGroup[*int]("integers")
		group.New("foo", &x)
	}, "Pointer should have caused a panic.", t)
}

func Test_Group_OnReadWrite(t *testing.T) {
	cycles := 100

	group := NewGroup[int]("group-1")
	seqPrevious := make([]int, 0)
	seqCurrent := make([]int, 0)

	groupName := ""
	sharefName := ""

	group.OnReadWrite(func(event ReadWriteEvent[int]) {
		seqPrevious = append(seqPrevious, *event.Previous)

		value := -1
		if event.Current != nil {
			value = *event.Current
		}
		seqCurrent = append(seqCurrent, value)

		groupName = event.GroupName
		sharefName = event.SharefName
	})

	sharef := group.New("sharef-1", 0)
	mutex := &sync.Mutex{}

	Concurrently(cycles, func() {
		mutex.Lock()
		defer mutex.Unlock()

		sharef.Do(func(portal Portal[int]) {
			pointer := <-portal.Reader
			value := *pointer
			value++
			portal.Writer <- &value
		})
	})

	assertSequential := func(firstValue int, sequence []int) {
		if sequence[0] != firstValue {
			t.Fatalf("First value in sequence doesn't match. Expected: '%d', actual: '%d'.", firstValue, sequence[0])
		}

		indexMax := len(sequence) - 1
		index := 0
		for {
			if index+1 > indexMax {
				break
			}

			if sequence[index+1] != sequence[index]+1 {
				t.Fatalf("Sequence is not sequential: '%v'.", sequence)
			}
			index++
		}
	}

	assertSequential(0, seqPrevious)
	assertSequential(1, seqCurrent)

	if groupName != "group-1" {
		t.Error("Incorrect group name.")
	}

	if sharefName != "sharef-1" {
		t.Error("Incorrect sharef name.")
	}
}
