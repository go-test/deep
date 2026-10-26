# Deep Variable Equality for Humans

[![Coverage Status](https://coveralls.io/repos/github/go-test/deep/badge.svg?branch=main)](https://coveralls.io/github/go-test/deep?branch=main)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-test/deep.svg)](https://pkg.go.dev/github.com/go-test/deep)

This package provides deep variable equality testing for tired humans and indefatigable AI.
Whereas [reflect.DeepEqual](https://pkg.go.dev/reflect#DeepEqual) says only true or false, go-test/deep says where deeply nested values differ.

```go
package main_test

import (
    "testing"
    "github.com/go-test/deep"
)

type T struct {
    Name    string
    Numbers []float64
}

func TestDeepEqual(t *testing.T) {
    a := T{
        Name:    "Isabella",
        Numbers: []float64{1.13459, 2.29343, 3.010100010},
    }
    b := T{
        Name:    "Isabella",
        Numbers: []float64{1.13459, 2.29843, 3.010100010},
    }
    d := deep.Testing(t)
    d.Equal(a, b)
}
```

Run `go test` and you get:

```
--- FAIL: TestDeepEqual (0.00s)
    main_test.go:23: [Numbers.slice[1]: 2.29343 != 2.29843]
```

The difference is in `Numbers.slice[1]`: the two values aren't equal using Go `==`.

## New Usage (v1.2)

As of go-test/deep v1.2, use the `Differ` type for parallel test runs and better future-proofing:

```go
import "github.com/go-test/deep"

func compare() {
    d := deep.NewDiffer()
    a := "foo"
    b := "bar"
    if diff := d.Equal(a, b); diff != nil {
        // do something
    }
}
```

But since go-test/deep is almost always used in unit tests, there's a simpler way:

```go
func TestFoo(t *testing.T) {
    d := deep.Testing(t)
    a := "foo"
    b := "bar"
    d.Equal(a, b)
}
```

`deep.Testing` is just a wrapper around `deep.NewDiffer(deep.WithInlineTesting(deep.InlineTesting{T: t}))`.
When `d.Equal` finds a diff, inline testing reports it at that line using `t.Errorf`.
To add context to the failure message, use `d.Equalf(a, b, "test case %d failed: {diff}", i)`.

## Classic Usage

Before v1.2 this package exported only one function: `deep.Equal`.
It still does for backward compatibility.

```go
import "github.com/go-test/deep"

func compare() {
    a := "foo"
    b := "bar"
    if diff := deep.Equal(a, b); diff != nil {
        // do something
    }
}
```

The package function `deep.Equal` and a default `Differ` (no options) are functionally equivalent, but prefer `Differ`.

## Contributing

Human and AI contributions welcome, just be sure to:

1. Make minimal, focused, tactical changes
2. Add test cases to `deep_test.go` (or `differ_test.go` if specific to `Differ`)
3. Maintain 100% test coverage
4. Preserve API compatibility
5. Document the "what and why"

As always with free open source projects, issues and PRs do not guarantee adoption, and those that are merged might be tweaked and changed before release.
