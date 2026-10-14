# Go style

House conventions for Go in this repo. They are about where declarations
and literals sit, not about formatting — `gofmt` still decides that.

## Hoist locals into a `var (...)` block

Declare every local at the top of the function (or of the scope that needs
it) in a single `var (...)` block, using `var name = expr` where there is an
initialiser. Inside `if` statements assign with `=` and inline the error
check:

```go
var (
	x   SomeType
	err error
)
if x, err = f(); err != nil {
	return err
}
```

The point is that reading the top of a function tells you everything it
works with and what type each thing is, and that `x` is still in scope after
the `if`. A `:=` inside the `if` would scope `x` to the branch and hide the
type at the point you want to see it.

Two exceptions:

- A goroutine closure declares its own `err` in an inner `var` block. It
  must not share the enclosing `err`, which would be a data race.
- `for _, v := range xs` iteration variables stay as `:=`.

## Struct literals are always multi-line

One field per line, with a trailing comma, even when there is a single
field:

```go
var x = MyStruct{
	Name: "abc",
}

// not
var y = MyStruct{Name: "abc"}
```

Adding or removing a field is then a one-line diff, and a literal never has
to be reflowed as it grows.

This holds when the struct exists only to call a method on it. Combined with
the `var` block rule, the construction appears inline in the `if`:

```go
var (
	r   Result
	err error
)
if r, err = (&MyStruct{
	Name: "abc",
}).SomeMethod(); err != nil {
	return nil, err
}
```

This shape is used throughout `samples/goohm/ast/build.go`, where each
visitor method builds a child's generated node struct and calls `Accept` on
it — see [grammar2ast.md](../architecture/grammar2ast.md).
