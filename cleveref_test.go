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

func TestImmutable_Forbid_Pointer(t *testing.T) {
	recovered := false
	var currentPanic cleveref.CleverefPanic

	defer func() {
		cleveref.Recover(recover(), func(p cleveref.CleverefPanic) {
			recovered = true
			currentPanic = p
		})

		if !recovered {
			t.Error("Should have panicked/recovered.")
		}

		if currentPanic != cleveref.PANIC_UNEXPECTED_POINTER {
			t.Error("Incorrect panic type.")
		}
	}()

	i := 10
	cleveref.NewImmutable[*int](&i)
}

func TestImmutable_Forbid_DoublePointer(t *testing.T) {
	recovered := false
	var currentPanic cleveref.CleverefPanic

	defer func() {
		cleveref.Recover(recover(), func(p cleveref.CleverefPanic) {
			recovered = true
			currentPanic = p
		})

		if !recovered {
			t.Error("Should have panicked/recovered.")
		}

		if currentPanic != cleveref.PANIC_UNEXPECTED_POINTER {
			t.Error("Incorrect panic type.")
		}
	}()

	a := 10
	b := &a
	c := &b
	cleveref.NewImmutable[**int](c)
}

func TestImmutable_Forbid_Map(t *testing.T) {
	recovered := false
	var currentPanic cleveref.CleverefPanic

	defer func() {
		cleveref.Recover(recover(), func(p cleveref.CleverefPanic) {
			recovered = true
			currentPanic = p
		})

		if !recovered {
			t.Error("Should have panicked/recovered.")
		}

		if currentPanic != cleveref.PANIC_UNEXPECTED_MAP {
			t.Error("Incorrect panic type.")
		}
	}()

	var m map[int]int
	cleveref.NewImmutable(m)
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
	// nested pointers bypass immutability, since pointers themselves
	// are mutable.
	// even if Immutable makes a copy, we're making a copy of a
	// pointer to the original memory location.
	// therefore the copy CAN modify the original contents.

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
