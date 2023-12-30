package sharedref

import (
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

func IncByValue(sharedref SharedRef[Counter]) {
	sharedref.Use(func(counter *Counter) {
		counter.IncByReference()
	})
}

func IncByReference(sharedref *SharedRef[Counter]) {
	sharedref.Use(func(counter *Counter) {
		counter.IncByReference()
	})
}

// Order represents the order in which a series of operations are
// performed; it is atomic, and thus, goroutine-safe.
type Order struct {
	s     []int
	mutex sync.Mutex
}

func NewOrder() Order {
	return Order{
		s:     make([]int, 0),
		mutex: sync.Mutex{},
	}
}

func (this *Order) Append(x int) {
	this.mutex.Lock()
	this.s = append(this.s, x)
	this.mutex.Unlock()
}

func (this *Order) Get() []int {
	return this.s
}

func (this *Order) Unique() []int {
	encountered := map[int]bool{}
	output := []int{}

	for _, value := range this.s {
		if encountered[value] == false {
			encountered[value] = true
			output = append(output, value)
		}
	}

	return output
}

// Concurrently runs a 'handler' function concurrently, 'times' times.
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

func Test_Dead_IsDead_IsAlive(t *testing.T) {
	sharedref := Dead[int]()

	if !sharedref.IsDead() {
		t.Error("SharedRef should be dead.")
	}

	if sharedref.IsAlive() {
		t.Error("SharedRef should be dead.")
	}
}

func Test_Pointer_Kills_SharedRef(t *testing.T) {
	a := 10
	sharedref := New(&a)

	if !sharedref.IsDead() {
		t.Error("SharedRef should be dead.")
	}

	if sharedref.IsAlive() {
		t.Error("SharedRef should be dead.")
	}
}

func Test_Use_Alive(t *testing.T) {
	value := 10
	sharedref := New(value)

	called := false
	result := sharedref.Use(func(ptr *int) {
		called = true

		if *ptr != value {
			t.Errorf("Value should've been: %d", value)
		}
	})

	if !called {
		t.Error("Use() did not call its handler.")
	}

	if !result {
		t.Error("Use() did not return true.")
	}
}

func Test_Use_Dead(t *testing.T) {
	sharedref := Dead[int]()

	called := false
	result := sharedref.Use(func(ptr *int) {
		called = true
	})

	if called {
		t.Error("Use() called its handler, even though the SharedRef was dead.")
	}

	if result {
		t.Error("Use() did not return false.")
	}
}

func Test_Use_Is_Not_Atomic(t *testing.T) {
	times := 100000
	sharedref := New(0)
	order := NewOrder()

	Concurrently(times, func() {
		used := sharedref.Use(func(ptr *int) {
			*ptr++
			order.Append(*ptr)
		})

		if !used {
			t.Error("Use() should have returned true.")
		}
	})

	if len(order.Get()) != times {
		t.Errorf("Order's length should be %d, instead it was %d", times, len(order.Get()))
	}

	// The lengths will be the same if Use() is calling its handler
	// atomically.
	if len(order.Unique()) == len(order.Get()) {
		t.Error("Use() demonstrated atomic behaviour on its own, when it shouldn't have.")
	}
}

func Test_Use_Inside_Use_Allowed(t *testing.T) {
	sharedref := New(0)

	a, b := false, false

	a = sharedref.Use(func(_ *int) {
		b = sharedref.Use(func(_ *int) {})
	})

	if !a || !b {
		t.Error("Use() should be allowed inside Use().")
	}
}

func Test_Use_Inside_Swap_Disallowed(t *testing.T) {
	sharedref := New(0)

	a, b := false, false

	a = sharedref.Swap(func(ptr *int) *int {
		b = sharedref.Use(func(_ *int) {})

		return ptr
	})

	if !a {
		t.Error("Swap() should have called its handler.")
	}

	if b {
		t.Error("Use() should not be allowed inside Swap().")
	}
}

func Test_Use_Inside_Locking_Allowed(t *testing.T) {
	sharedref := New(0)

	a, b := false, false

	a = sharedref.Locking(func() {
		b = sharedref.Use(func(_ *int) {})
	})

	if !a || !b {
		t.Error("Use() should be allowed inside Locking().")
	}
}

func Test_Locking_Alive(t *testing.T) {
	value := 10
	sharedref := New(value)

	called := false
	result := sharedref.Locking(func() {
		called = true
	})

	if !called {
		t.Error("Locking() did not call its handler.")
	}

	if !result {
		t.Error("Locking() did not return true.")
	}
}

func Test_Locking_Dead(t *testing.T) {
	sharedref := Dead[int]()

	called := false
	result := sharedref.Locking(func() {
		called = true
	})

	if called {
		t.Error("Locking() called its handler, even though the SharedRef was dead.")
	}

	if result {
		t.Error("Locking() did not return false.")
	}
}

func Test_Locking_Is_Atomic(t *testing.T) {
	times := 100000
	order := NewOrder()

	deadlocked := false
	onContention := func(elapsed time.Duration, giveUp func()) {
		if elapsed >= time.Second {
			deadlocked = true
			giveUp()
		}
	}

	sharedref := New(0, onContention)

	Concurrently(times, func() {
		calledUse := sharedref.Use(func(ptr *int) {
			calledLocking := sharedref.Locking(func() {
				*ptr++
				order.Append(*ptr)
			})

			if !calledLocking {
				t.Error("Locking() should have returned true.")
			}
		})

		if !calledUse {
			t.Error("Use() should have returned true.")
		}
	})

	if len(order.Get()) != times {
		t.Errorf("Order's length should be %d, instead it was %d", times, len(order.Get()))
	}

	// The lengths will be different if Locking() is not calling its
	// handler atomically.
	if len(order.Unique()) != times {
		t.Error("Locking() did not demonstrate atomic behaviour.")
	}

	if deadlocked {
		t.Error("Locking() deadlocked.")
	}
}

func Test_Locking_Inside_Use_Allowed(t *testing.T) {
	sharedref := New(0)

	a, b := false, false

	a = sharedref.Use(func(_ *int) {
		b = sharedref.Locking(func() {})
	})

	if !a || !b {
		t.Error("Locking() should be allowed inside Use().")
	}
}

func Test_Locking_Inside_Swap_Disallowed(t *testing.T) {
	sharedref := New(0)

	a, b := false, false

	a = sharedref.Swap(func(ptr *int) *int {
		b = sharedref.Locking(func() {})

		return ptr
	})

	if !a {
		t.Error("Swap() should have called its handler.")
	}

	if b {
		t.Error("Locking() should not be allowed inside Swap().")
	}
}

func Test_Locking_Inside_Locking_Deadlocks(t *testing.T) {
	deadlocked := false
	onContention := func(elapsed time.Duration, giveUp func()) {
		if elapsed >= time.Second {
			deadlocked = true
			giveUp()
		}
	}

	sharedref := New(0, onContention)

	b := false
	a := sharedref.Locking(func() {
		b = sharedref.Locking(func() {})
	})

	if !a {
		t.Error("Locking() should have called its handler.")
	}

	if !deadlocked {
		t.Error("Locking() inside Locking() should have caused a deadlock.")
	}

	if b {
		t.Error("Nested Locking() should not have called its handler, as it deadlocked.")
	}
}

func Test_Swap_Alive(t *testing.T) {
	value := 10
	sharedref := New(value)

	called := false
	result := sharedref.Swap(func(ptr *int) *int {
		called = true

		if *ptr != value {
			t.Errorf("Value should've been: %d", value)
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

func Test_Swap_Dead(t *testing.T) {
	sharedref := Dead[int]()

	called := false
	result := sharedref.Swap(func(ptr *int) *int {
		called = true
		return ptr
	})

	if called {
		t.Error("Swap() called its handler, even though the SharedRef was dead.")
	}

	if result {
		t.Error("Swap() did not return false.")
	}
}

func Test_Swap_Is_Atomic(t *testing.T) {
	times := 100000
	sharedref := New(0)

	Concurrently(times, func() {
		called := sharedref.Swap(func(ptr *int) *int {
			v := *ptr
			v++
			return &v
		})

		if !called {
			t.Error("Swap() should have returned true.")
		}
	})

	sharedref.Use(func(ptr *int) {
		if *ptr != times {
			t.Error("Swap() did not demonstrate atomic behaviour.")
		}
	})
}

func Test_Swap_Nil_Pointer_Kills_SharedRef(t *testing.T) {
	sharedref := New(10)

	sharedref.Swap(func(ptr *int) *int {
		return nil
	})

	if !sharedref.IsDead() {
		t.Error("SharedRef should be dead.")
	}
}

func Test_Swap_Inside_Use_Disallowed(t *testing.T) {
	sharedref := New(10)

	b := true
	a := sharedref.Use(func(_ *int) {
		b = sharedref.Swap(func(ptr *int) *int {
			return ptr
		})
	})

	if !a {
		t.Error("Use() should have called its handler.")
	}

	if b {
		t.Error("Swap() should not be allowed inside Use().")
	}
}

func Test_Swap_Inside_Locking_Disallowed(t *testing.T) {
	sharedref := New(10)

	b := true
	a := sharedref.Locking(func() {
		b = sharedref.Swap(func(ptr *int) *int {
			return ptr
		})
	})

	if !a {
		t.Error("Locking() should have called its handler.")
	}

	if b {
		t.Error("Swap() should not be allowed inside Locking().")
	}
}

func Test_Swap_Inside_Swap_Deadlocks(t *testing.T) {
	deadlocked := false
	onContention := func(elapsed time.Duration, giveUp func()) {
		if elapsed >= time.Second {
			deadlocked = true
			giveUp()
		}
	}

	sharedref := New(0, onContention)

	b := false
	a := sharedref.Swap(func(ptr *int) *int {
		b = sharedref.Swap(func(nptr *int) *int {
			return nptr
		})

		return ptr
	})

	if !a {
		t.Error("Swap() should have called its handler.")
	}

	if !deadlocked {
		t.Error("Swap() inside Swap() should have caused a deadlock.")
	}

	if b {
		t.Error("Nested Swap() should not have called its handler, as it deadlocked.")
	}
}

func Test_IsLocked(t *testing.T) {
	sharedref := New(10)

	if sharedref.IsLocked() {
		t.Error("SharedRef should not be locked.")
	}

	sharedref.Use(func(_ *int) {
		if sharedref.IsLocked() {
			t.Error("SharedRef should not be locked inside Use().")
		}
	})

	sharedref.Locking(func() {
		if !sharedref.IsLocked() {
			t.Error("SharedRef should be locked inside Locking().")
		}
	})

	sharedref.Swap(func(ptr *int) *int {
		if !sharedref.IsLocked() {
			t.Error("SharedRef should be locked inside Swap().")
		}

		return ptr
	})
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
	sharedref := New(counter)

	// Call methods directly inside a Use() block. Regular Go
	// semantics apply.
	sharedref.Use(func(ptr *Counter) {
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

	// Call methods inside another function that received the
	// SharedRef as a copy.
	// Value becomes 2.
	IncByValue(sharedref)
	sharedref.Use(func(ptr *Counter) {
		if ptr.Value != 2 {
			t.Error("Function IncBySharedRefValue() performed no mutation.")
		}
	})

	// Call methods inside another function that received the
	// SharedRef by reference.
	// Value becomes 3.
	IncByReference(&sharedref)
	sharedref.Use(func(ptr *Counter) {
		if ptr.Value != 3 {
			t.Error("Function IncBySharedRefReference() performed no mutation.")
		}
	})

	// Swap() directly on the 'sharedref' symbol. Mutates.
	// Value becomes 4.
	sharedref.Swap(func(ptr *Counter) *Counter {
		counter := *ptr
		counter.Value++
		return &counter
	})

	func(copy SharedRef[Counter]) {
		// Swap() on a local copy named 'copy'. Mutates.  Value
		// becomes 5.
		copy.Swap(func(ptr *Counter) *Counter {
			counter := *ptr
			counter.Value++
			return &counter
		})
	}(sharedref)

	// We can see the original 'sharedref' was mutated here.
	sharedref.Use(func(ptr *Counter) {
		counter := *ptr
		if counter.Value != 5 {
			t.Error("Swap() performed no mutations.")
		}
	})
}

func Test_SliceExtract(t *testing.T) {
	input := []SharedRef[int]{New(1), New(2), New(3)}
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
