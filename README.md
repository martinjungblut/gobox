## cleveref

### clever/smart references for the Go programming language

----

This library is part of the GoExt project. It aims at making Go suck less. Go sucks like a vacuum cleaner.
Don't get me wrong, Go is a great tool. I've been writing it for 8 years. It's good. But it sucks.

It's like when you hold some money bills. It's money, it's good. But now your hands are all dirty, you need to wash your hands, you can just feel the filth.
That's Go. Don't pretend it isn't, don't pretend you have a Rob Pike poster in your bedroom and this is the ultimate end of programming language design and evolution.

No hate on Rob Pike. He's wonderful. This is a good language, if it weren't, we wouldn't be building on it (errrr JavaScript **cough**).

Embrace the filth! But let's make it suck less, please.

#### Why am I even doing this?

Just like C, Go passes everything by value (as a copy).
If you want to reference some data, you pass a pointer to it. But the pointer itself is passed by value.
It's the semantics of C.

> So what happens when you're writing Go at scale?

What happens is that methods operate on receivers. A receiver is the thing upon which a method operates (like `this` in C++/C#/Java, or `self` in Python).

Methods may declare their receiver as a value (of type T), or as a pointer (of type *T).
A method that declares its receiver as a pointer can modify the struct upon which it operates.
The pointer is passed by copy, but it points to the underlying allocated struct.
If the struct is passed by value, then it's a full copy, and modifications only modify the copy, which is discarded when the method finishes executing.

Ok, so what?
Well, here's the thing:

> Looking at a method alone, we can never determine whether it's actually mutating something or not. It may be mutating a struct that's long-lived. It may be mutating a struct that's only local to a function, and that was implicitly created when it was being copied as a parameter on the call stack.

This makes mutations hard to analyse at a glance:

```go
type K struct {
	// public int
	Counter int
}

// By looking at IncMethod() alone, we can't determine if we're mutating the original object,
// or a copy that was implicitly created along the call stack.
func (this *K) IncMethod() {
	this.Counter++
}

func IncFunc(k K) {
	// Would mutate! You'd think so.
	// But 'k' was passed as a struct copy, so the integer is copied too.
	// We're mutating the copy.
    k.IncMethod()
}

func main() {
	k := K{Counter: 0}
	k.IncMethod()  // Mutates.
	IncFunc(k)     // Does not mutate.
	// k.Counter is still 1 here
}
```

There are various way to do copies in Go, as a side effect of simply trying to get types to match so that the program actually _compiles_.

And this creates a maintenance burden. Behold:

```go
package main

import "fmt"

type Counter struct {
	n int
}

func (this *Counter) Inc() {
	this.n++
}

func (this Counter) Display() {
	fmt.Printf("n = %d\n", this.n)
}

type CounterHolder struct {
	C *Counter
}

func (this CounterHolder) Inc() {
	this.C.Inc()
}

func A(flag int, target *Counter) {
	if flag == 0 {
		target.Inc()
	} else if flag == 1 {
		// oops, made a copy when we dereferenced the pointer
		copy := *target
		copy.Inc()
	} else {
		B(CounterHolder{C: target})
	}
}

func B(target CounterHolder) {
	target.Inc()
}

func main() {
	c := Counter{}

	A(0, &c)
	c.Display() // mutates, prints 1

	A(1, &c)
	c.Display() // doesn't mutate, prints 1

	A(2, &c)
	c.Display() // mutates, prints 2
}
```

**What if we had forgotten to declare C as a pointer?**

```go
type CounterHolder struct {
	C Counter
}
```

Well, then the code wouldn't compile, rightfully so, because of this line: `B(CounterHolder{C: target})`

```
./prog.go:33:22: cannot use target (variable of type *Counter) as Counter value in struct literal
```

You may be tempted to dereference the pointer to satisfy the type checker, but now you just made a copy, by mistake: `B(CounterHolder{C: *target})`

That last mutation (that printed 2) will never be triggered.

**What if we had forgotten to declare `this` as a pointer, for the `Counter.Inc()` method?**

```go
func (this Counter) Inc() {
	this.n++
}
```

Well, now no mutation will ever create any meaningful state change. As we're always mutating copies, we'll just print 0 three times.

**In a nutshell, Go requires us to be mindful of: struct members' types, function/method type definitions, call sites and receiver type definitions.**

And because programs tend to evolve over time to interweave types based on specific needs, it's very often hard to determine when we're mutating a copy or not, particularly with deeply nested call stacks.

#### The plan

> Let's go, let's suck the suckage out of Go.

To suck the suckage out of Go, we'll introduce some special types.
These special types will make our lives easier. Hopefully.

This is the list of special types currently provided by `cleveref`:

- `Immutable`.

That's it for now. Stay tuned, kids!

#### Immutable

`Immutable` is a type that prevents us from doing those struct copies implicitly.

It forces us not to use pointers, maps, and other mutables types.

It also imposes an API to access values, and values are always accessed by copying, as pointers are not allowed.

`Immutable` works with:

- All Go primitive types: bool, string, all number types, bytes and runes.
- Structs.
- Slices. Althought slices are mutable, when we pass a slice to a function (which we must do, when using `Immutable`), the copied reference cannot change the original slice's _view_ of its designated memory area. So even though slices are mutable in Go, when we shim them through `Immutable`, in practice, they become immutable. Nice!

`Immutable` does not work with:

- Pointers, since they're mutable, and that's what we're trying to avoid. Dereference your pointers and pass their values to `Immutable`.

- Maps. Maps are reference types in Go, meaning they're always mutable, and even when passed around as a copy, the map is still pointing to its underlying allocated memory. To circumvent this limitation, we'll introduce an `ImmutableMap` in the future.

- If you provide either a pointer or a map, a dead `Immutable` will be produced. It's useless. You can check if an `Immutable` is dead by calling `IsDead()`.

This being Go, there are some gotchas:

- Structs containing pointers: if a struct contains a pointer, even if the struct itself is wrapped as an `Immutable`, any copies of said struct would still hold a copy of a pointer that references the same memory location as the original struct. Therefore, they cannot be feasibly immutable in Go. But you can always use `Immutable` inside structs. At least now you have a proper type to `grep` for.

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

```go
// create an immutable int
i := NewImmutable(5)

// and Swap() it
k := i.Swap(func(x int) int {
	// k will be Immutable[int](10)
	return x * 2
})

i.Use(func(x int) {
	fmt.Printf("x will be 5: %d\n", x)
})

k.Use(func(x int) {
	fmt.Printf("x will be 10: %d\n", x)
})
```
