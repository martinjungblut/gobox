package cleveref_test

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"cleveref"
)

type SymbolTable struct {
	Value         int
	Pointer       *int
	Slice         []int
	Map           map[int]int
	StructPointer *SymbolTable
}

// Unless symbolTable is wrapped as an Immutable, it will be mutated
func (s *SymbolTable) SetPointer(newPointer *int) {
	s.Pointer = newPointer
}

func TestImmutable_Use(t *testing.T) {
	var mut int = 0

	immut := cleveref.NewImmutable[int](10)
	immut.Use(func(v int) {
		mut += v
	})

	if mut != 10 {
		t.Error("Mut should be 10.")
	}
}

func TestImmutable_Swap(t *testing.T) {
	first := cleveref.NewImmutable(10)
	second := first.Swap(func(i int) int {
		return i * 2
	})

	firstCounter := 0
	first.Use(func(i int) {
		firstCounter += i
	})
	if firstCounter != 10 {
		t.Error("First immutable didn't contain the expected value.")
	}

	secondCounter := 0
	second.Use(func(i int) {
		secondCounter += i
	})
	if secondCounter != 20 {
		t.Error("Second immutable didn't contain the expected value.")
	}
}

func TestImmutable_Dead_Use(t *testing.T) {
	called := false

	i := 10
	immut := cleveref.NewImmutable[*int](&i)
	immut.Use(func(_ *int) {
		called = true
	})

	if called {
		t.Error("Dead immutables should never allow Use().")
	}
}

func TestImmutable_Dead_Swap(t *testing.T) {
	i := 10
	first := cleveref.NewImmutable[*int](&i)

	called := false
	second := first.Swap(func(_ *int) *int {
		called = true
		return nil
	})

	if called {
		t.Error("Dead immutables should never allow Swap().")
	}

	if !second.IsDead() {
		t.Error("Calling Swap() on dead immutables should always return dead immutables.")
	}
}

func TestImmutable_Dead_Pointer(t *testing.T) {
	i := 10

	immut := cleveref.NewImmutable[*int](&i)
	if !immut.IsDead() {
		t.Error("Immutable should be dead.")
	}
}

func TestImmutable_Dead_DoublePointer(t *testing.T) {
	a := 10
	b := &a
	c := &b

	immut := cleveref.NewImmutable[**int](c)
	if !immut.IsDead() {
		t.Error("Immutable should be dead.")
	}
}

func TestImmutable_Dead_Map(t *testing.T) {
	var m map[int]int

	immut := cleveref.NewImmutable(m)
	if !immut.IsDead() {
		t.Error("Immutable should be dead.")
	}
}

func TestImmutable_Immutability_Slice(t *testing.T) {
	s := []int{10, 20, 30}

	// Immutable slice
	func() {
		immut := cleveref.NewImmutable(s)
		immut.Use(func(slice []int) {
			slice = append(slice, 40)
			slice[0] = 40
		})

		if len(s) != 3 {
			t.Error("Slice mutated.")
		}

		if s[0] != 10 {
			t.Error("Slice mutated.")
		}
	}()

	// Immutable struct containing slice
	func() {
		immut := cleveref.NewImmutable(SymbolTable{Slice: s})
		immut.Use(func(table SymbolTable) {
			table.Slice = append(table.Slice, 40)
			table.Slice[0] = 40
		})

		if len(s) != 3 {
			t.Error("Slice mutated.")
		}

		if s[0] != 10 {
			t.Error("Slice mutated.")
		}
	}()
}

func TestImmutable_Immutability_Struct(t *testing.T) {
	a, b := 10, 20
	table := SymbolTable{Value: a, Pointer: &a}

	immut := cleveref.NewImmutable(table)
	immut.Use(func(t SymbolTable) {
		t.Value = b
		t.Pointer = &b
	})

	if table.Value != a {
		t.Error("Table mutated: value is no longer the same.")
	}

	if table.Pointer != &a {
		t.Error("Table mutated: pointer is no longer the same.")
	}
}

func TestImmutable_Immutability_StructMethod(t *testing.T) {
	a, b := 10, 20
	table := SymbolTable{Value: a, Pointer: &a}

	immut := cleveref.NewImmutable(table)
	immut.Use(func(t SymbolTable) {
		// As SetPointer is a pass-by-reference method, it would
		// mutate 'table', in run-of-the-mill Go.
		// But 't' is a copy of 'table', therefore no mutation is
		// actually performed. The copy 't' is mutated, but not
		// 'table'.
		t.SetPointer(&b)
	})

	if table.Pointer != &a {
		t.Error("Table mutated: pointer is no longer the same.")
	}
}

func TestImmutable_Immutability_Bypass(t *testing.T) {
	// Nested pointers bypass immutability, since pointers themselves
	// are mutable.
	// Even if Immutable makes a copy, we're making a copy of a
	// pointer to the original memory location.
	// Therefore the copy CAN modify the original contents.

	a, b := 10, 20
	owned := SymbolTable{Value: a, Pointer: &a}
	owner := SymbolTable{StructPointer: &owned, Pointer: &a}

	immut := cleveref.NewImmutable(owner)
	immut.Use(func(t SymbolTable) {
		t.StructPointer.Value = b
		t.StructPointer.Pointer = &b
	})
	if owner.StructPointer.Value == a {
		t.Error("Nested table did not mutate. It should have.")
	}
	if owner.StructPointer.Pointer == &a {
		t.Error("Nested table did not mutate. It should have.")
	}

	immut.Use(func(t SymbolTable) {
		*t.Pointer = 50
	})
	if a != 50 {
		t.Error("Integer did not mutate. It should have.")
	}
}

func TestAtom_DoublePointer(t *testing.T) {
	a := 10
	b := &a
	c := &b
	atom := cleveref.NewAtom(c)

	if !atom.IsDead() {
		t.Error("Atom should be dead.")
	}

	// Use() should never invoke its continuation if the Atom is dead
	called := false
	atom.Use(func(v **int) {
		called = true
	})
	if called {
		t.Error("Use() should not have called its continuation.")
	}
}

func TestAtom_Pointer(t *testing.T) {
	for i := 0; i < 5; i++ {
		called := false

		atom := cleveref.NewAtom(&i)
		atom.Use(func(value *int) {
			called = true

			if *value != i {
				t.Errorf("Value should've been: %d", i)
			}
		})

		if !called {
			t.Error("Use() did not call its continuation.")
		}
	}
}

type Counter struct {
	Value int
}

func (this *Counter) RefInc() {
	this.Value++
}

func (this Counter) Inc() {
	this.Value++
}

func FuncInc(atom cleveref.Atom[Counter]) {
	atom.Use(func(counter *Counter) {
		counter.RefInc()
	})
}

func TestAtom_Use(t *testing.T) {
	// establish truths
	counter := Counter{Value: 0}

	counter.RefInc()
	if counter.Value != 1 {
		t.Error("RefInc() performed no mutation.")
	}

	counter.Inc()
	if counter.Value != 1 {
		t.Error("Inc() performed a mutation.")
	}

	// Call methods directly inside a Use() block
	atom := cleveref.NewAtom(&counter)
	atom.Use(func(c *Counter) {
		c.RefInc()
		if c.Value != 2 {
			t.Error("RefInc() performed no mutation.")
		}

		c.Inc()
		if c.Value != 2 {
			t.Error("Inc() performed a mutation.")
		}
	})

	// Call methods inside another function that received the Atom as
	// a copy
	FuncInc(atom)
	atom.Use(func(c *Counter) {
		if c.Value != 3 {
			t.Error("FuncInc() performed no mutation.")
		}
	})
}

func TestAtom_IsAtomic(t *testing.T) {
	maxprocs := runtime.NumCPU() + 1

	fmt.Printf("Setting GOMAXPROCS to %d.\n", maxprocs)
	runtime.GOMAXPROCS(maxprocs)

	cycles := 1000000

	i := 0
	atom := cleveref.NewAtom(&i)

	var wg sync.WaitGroup

	for i := 1; i <= cycles; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			atom.Use(func(number *int) {
				*number++
				// *number = *number + 1
			})
		}()
	}

	wg.Wait()

	atom.Use(func(number *int) {
		if *number != cycles {
			t.Errorf("Atom is not atomic. Value was %d, should have been %d.", *number, cycles)
		}
	})
}

func TestAtom_Hijack(t *testing.T) {

}
