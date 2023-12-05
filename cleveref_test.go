package cleveref_test

import (
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

	if !first.IsDead() {
		t.Error("First immutable should be dead.")
	}

	counter := 0
	second.Use(func(i int) {
		counter += i
	})
	if counter != 20 {
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
	called := false

	i := 10
	first := cleveref.NewImmutable[*int](&i)
	second := first.Swap(func(_ *int) *int {
		called = true
		return nil
	})

	if called {
		t.Error("Dead immutables should never allow Swap().")
	}

	if !second.IsDead() {
		t.Error("Swap() on a dead immutable should always return a dead immutable.")
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
