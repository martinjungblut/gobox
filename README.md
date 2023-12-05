### cleveref

#### Clever References for the Go programming language

This library is part of the GoExt project. It aims at making Go suck less. Go sucks like a vacuum cleaner.

`Immutable` works with:

- All Go primitive types: bool, string, all number types, bytes and runes.
- Structs.
- Slices.

`Immutable` does not work with:

- Pointers, since they're mutable, and that's what we're trying to avoid. Dereference your pointers and pass their values to `Immutable`.

- Maps. Maps are reference types in Go, meaning they're always mutable, and even when passed around as a copy, the map is still pointing to its underlying allocated memory. To circumvent this limitation, we'll introduce an `ImmutableMap` in the future.

And there are some gotchas:

- Structs containing pointers: if a struct contains a pointer, even if the struct itself is wrapped as an `Immutable`, any copíes of said struct would still hold a copy of a pointer that references the same memory location as the original struct. Therefore, they cannot be feasibly immutable in Go. But you can always use `Immutable` inside structs!

#### Example #1: Use()

`Immutable` creates automatic copies whenever you `Use()` it:

```go
// create an immutable int
i := NewImmutable(5)

// and Use() it
i.Use(func(x int) {
	fmt.Printf("We are going to print 5! %d\n", x)
	
	// x is a copy, i will still be 5
	x += 7
}) 

// i is still 5 here
```

#### Example #2: Swap()

Swap() allows an `Immutable` to produce another `Immutable`. You may then replace the original reference.

The original `Immutable` will be dead after that point. Dead immutables may no longer call `Use()` or `Swap()`!

```go
// create an immutable int
i := NewImmutable(5)

// and Use() it
k := i.Swap(func(x int) {
	// k will be Immutable[int](10)
	return x * 2
}) 

// i is dead now!
i.IsDead() == true

// i.Use() and i.Swap() will do nothing, but we can still use k, since it's alive
k.Use(func(x int) {
	fmt.Printf("x will be 10: %d\n", x)
})
```
