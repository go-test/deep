// Package deep provides function deep.Equal which is like reflect.DeepEqual
// but returns a descriptive set of differences. This surfaces differences in
// deeply nested data structures. It also has options for pragmatic equality,
// such as treating empty and nil slices as equal. And it works with testing.T
// to test and report diffs in a single line.
//
// As of v1.2, Differ is canonical usage rather than package function Equal.
// See the Differ type docs for usage.
package deep

import (
	"fmt"
	"reflect"
	"strings"
)

type Opt func(*Differ)

// WithFloatPrecision is the number of decimal places to round float values
// to when comparing.
func WithFloatPrecision(p uint) Opt {
	return func(d *Differ) {
		d.floatPrecision = p
	}
}

// WithMaxDiff specifies the maximum number of differences to return.
func WithMaxDiff(m uint) Opt {
	return func(d *Differ) {
		d.maxDiff = m
	}
}

// WithMaxDepth specifies the maximum levels of a struct to recurse into,
// if greater than zero. If zero, there is no limit.
func WithMaxDepth(m uint) Opt {
	return func(d *Differ) {
		d.maxDepth = m
	}
}

// WithLogErrors causes errors to be logged to STDERR when true.
func WithLogErrors(b bool) Opt {
	return func(d *Differ) {
		d.logErrors = b
	}
}

// WithCompareUnexportedFields causes unexported struct fields, like s in
// T{s int}, to be compared when true. This does not work for comparing
// error or Time types on unexported fields because methods on unexported
// fields cannot be called.
func WithCompareUnexportedFields(b bool) Opt {
	return func(d *Differ) {
		d.compareUnexportedFields = b
	}
}

// WithCompareFunctions compares functions the same as reflect.DeepEqual:
// only two nil functions are equal. Every other combination is not equal.
// This is disabled by default because previous versions of this package
// ignored functions. Enabling it can possibly report new diffs.
func WithCompareFunctions(b bool) Opt {
	return func(d *Differ) {
		d.compareFunctions = b
	}
}

// WithNilSlicesAreEmpty causes a nil slice to be equal to an empty slice.
func WithNilSlicesAreEmpty(b bool) Opt {
	return func(d *Differ) {
		d.nilSlicesAreEmpty = b
	}
}

// WithNilMapsAreEmpty causes a nil map to be equal to an empty map.
func WithNilMapsAreEmpty(b bool) Opt {
	return func(d *Differ) {
		d.nilMapsAreEmpty = b
	}
}

// WithNilPointersAreZero causes a nil pointer to be equal to a zero value.
func WithNilPointersAreZero(b bool) Opt {
	return func(d *Differ) {
		d.nilPointersAreZero = b
	}
}

// WithIgnoreSliceOrder causes Equal to ignore slice order so that
// []int{1, 2} and []int{2, 1} are equal. Only slices of primitive scalars
// like numbers and strings are supported. Slices of complex types,
// like []T where T is a struct, are undefined because Equal does not
// recurse into the slice value when this flag is enabled.
func WithIgnoreSliceOrder(b bool) Opt {
	return func(d *Differ) {
		d.ignoreSliceOrder = b
	}
}

// WithInlineTesting makes Equal and Equalf call T.Errorf when diffs are found.
// It also calls T.Helper to make the error report where it's called in the
// unit test, not within Equal/Equalf. This option replaces a common pattern:
//
//	if diff := deep.Equal("foo", "bar"); diff != nil {
//	    t.Error(diff)
//	}
//
// Instead, when d is a Differ with inline testing:
//
//	d.Equal("foo", "bar")
//
// Or use Equalf to format the test failure output.
func WithInlineTesting(i InlineTesting) Opt {
	return func(d *Differ) { d.inline = i }
}

// InlineTesting is the value passed to WithInlineTesting. Only T is required,
// and notice: it's the deep.T interface, and standard library testing.T
// satisfies this interface. (The deep.T interface is necessary to test the
// testing.)
//
// If specially formatted test failure output is not needed, deep.Testing
// quickly instantiates an inline testing Differ.
type InlineTesting struct {
	T             T                 // *testing.T (required)
	Fatal         bool              // call T.Errorf by default
	DiffFormatter func(Diff) string // replaces DiffPlaceholder
}

// DiffPlaceholder is replaced by the formatted diff in the Equalf format.
const DiffPlaceholder = `{diff}`

// Differ diffs two values and returns a Diff if any are found, else nil.
// The easiest usage is with inline testing:
//
//	func TestFoo(t *testing.T) {
//	    d := deep.Testing(t)
//	    a := "foo"
//	    b := "bar"
//	    d.Equal(a, b)
//	}
//
// That unit test will t.Error at the d.Equal line because "foo" != "bar".
// For greater control of the Differ, call NewDiffer with options. If the old
// style (before v1.2) is preferred but parallel Differ are required:
//
//	d := deep.NewDiffer()
//	if diff := d.Equal(a, b); diff != nil {
//	    t.Error(diff)
//	}
//
// That is functionally equivalent to using the original package function:
// deep.Equal(a, b), which is preserved because it's still widely used.
type Differ struct {
	compareFunctions        bool
	compareUnexportedFields bool
	floatPrecision          uint
	ignoreSliceOrder        bool
	logErrors               bool
	maxDepth                uint
	maxDiff                 uint
	nilMapsAreEmpty         bool
	nilPointersAreZero      bool
	nilSlicesAreEmpty       bool
	inline                  InlineTesting
}

// NewDiffer creates a new Differ with default values if no options are given.
// Differ defaults are equivalent to package function Equal.
func NewDiffer(opts ...Opt) Differ {
	d := Differ{
		// options where zero-value equals default value are omitted
		floatPrecision: 10,
		maxDiff:        10,
	}
	for opt := range opts {
		opts[opt](&d)
	}
	return d
}

// Testing returns an inline testing Differ. It's a convenience wrapper for:
//
//	d := deep.NewDiffer(
//	    deep.WithInlineTesting(deep.InlineTesting{
//	        T: t,
//	    }),
//	)
func Testing(t T) Differ {
	return NewDiffer(WithInlineTesting(InlineTesting{T: t}))
}

// Diff is a set of differences between a and b, or nil if no diffs were found.
type Diff []string

// Equal compares a and b and returns a Diff if any are found, else nil.
// With inline testing, the diff is reported. Without inline testing, the
// caller must check if diff != nil and handle if true (e.g. cause a test
// failure with t.Error).
//
// With default options, it recurses without limit and compares every exported
// field, but it stops early after finding WithMaxDiff diffs (default: 10).
//
// When comparing a struct, fields tagged `deep:"-"` are ignored.
//
// If a type has an Equal() bool method, like time.Equal, it's called to check
// for equality.
//
// Some differences may not be found if an error occurs.
func (d Differ) Equal(a, b any) Diff {
	if d.inline.T != nil {
		d.inline.T.Helper()
	}
	return d.Equalf(a, b, DiffPlaceholder)
}

// Equalf is Equal with custom error message formatting. The format and args
// are passed to T.Errorf/T.Fatalf with only one change: DiffPlaceholder is
// replaced with the formatted diff. For example:
//
//	d.Equalf(a, b, "test case %d failed: {diff}", i)
//
// Values a and b are not args to the format; they're the values being tested.
// The resulting diff, if any, is represented in the format by DiffPlaceholder.
// Use InlineTesting.DiffFormatter to format the diff before interpolation.
func (d Differ) Equalf(a, b any, format string, args ...any) Diff {
	diff := d.diff(a, b)
	if diff == nil {
		return nil
	}

	t := d.inline.T
	if t == nil {
		return diff
	}

	// Format the diff then interpolate with placeholder. Must also s/%/%%/g
	// in the diff string so they're literal % in Errorf/Fatalf rather than
	// direcctives.
	var s string
	if d.inline.DiffFormatter != nil {
		s = d.inline.DiffFormatter(diff)
	} else {
		s = fmt.Sprintf("%s", diff)
	}
	format = strings.ReplaceAll(format, DiffPlaceholder, strings.ReplaceAll(s, `%`, `%%`))

	// Report diff at caller's line number
	t.Helper()
	if d.inline.Fatal {
		t.Fatalf(format, args...)
	} else {
		t.Errorf(format, args...)
	}

	return diff
}

// diff returns the set of differences between a and b, or nil if none found.
// With default options, it recurses without limit and compares every exported
// field, but it stops early after finding WithMaxDiff diffs (default: 10).
//
// When comparing a struct, fields tagged `deep:"-"` are ignored.
//
// Some differences may not be found if an error occurs.
func (d Differ) diff(a, b any) Diff {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)
	c := &cmp{
		conf:        d,
		diff:        []string{},
		buff:        []string{},
		floatFormat: fmt.Sprintf("%%.%df", d.floatPrecision),
	}

	// Can bypass full diff if both or only one operand is nil
	if a == nil && b == nil {
		return nil
	} else if a == nil && b != nil {
		c.saveDiff("<nil pointer>", b)
		goto ret
	} else if a != nil && b == nil {
		c.saveDiff(a, "<nil pointer>")
		goto ret
	}

	c.equals(aVal, bVal, 0) // full diff

ret:
	if len(c.diff) > 0 {
		return c.diff // diffs
	}

	return nil // no diffs
}

// T is the subset of standard library testing.T that WithInlineTesting needs.
type T interface {
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Helper()
}
