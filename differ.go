// Package deep provides function deep.Equal which is like reflect.DeepEqual but
// returns a list of differences. This is helpful when comparing complex types
// like structures and maps.
package deep

import (
	"fmt"
	"reflect"
)

type Opt func(*Differ) error

// WithFloatPrecision is the number of decimal places to round float values
// to when comparing.
func WithFloatPrecision(p int) Opt {
	return func(d *Differ) error {
		d.floatPrecision = p
		return nil
	}
}

// WithMaxDiff specifies the maximum number of differences to return.
func WithMaxDiff(m int) Opt {
	return func(d *Differ) error {
		d.maxDiff = m
		return nil
	}
}

// WithMaxDepth specifies the maximum levels of a struct to recurse into,
// if greater than zero. If zero, there is no limit.
func WithMaxDepth(m int) Opt {
	return func(d *Differ) error {
		d.maxDepth = m
		return nil
	}
}

// WithLogErrors causes errors to be logged to STDERR when true.
func WithLogErrors(b bool) Opt {
	return func(differ *Differ) error {
		differ.logErrors = b
		return nil
	}
}

// WithCompareUnexportedFields causes unexported struct fields, like s in
// T{s int}, to be compared when true. This does not work for comparing
// error or Time types on unexported fields because methods on unexported
// fields cannot be called.
func WithCompareUnexportedFields(b bool) Opt {
	return func(differ *Differ) error {
		differ.compareUnexportedFields = b
		return nil
	}
}

// WithCompareFunctions compares functions the same as reflect.DeepEqual:
// only two nil functions are equal. Every other combination is not equal.
// This is disabled by default because previous versions of this package
// ignored functions. Enabling it can possibly report new diffs.
func WithCompareFunctions(b bool) Opt {
	return func(differ *Differ) error {
		differ.compareFunctions = b
		return nil
	}
}

// WithNilSlicesAreEmpty causes a nil slice to be equal to an empty slice.
func WithNilSlicesAreEmpty(b bool) Opt {
	return func(differ *Differ) error {
		differ.nilSlicesAreEmpty = b
		return nil
	}
}

// WithNilMapsAreEmpty causes a nil map to be equal to an empty map.
func WithNilMapsAreEmpty(b bool) Opt {
	return func(differ *Differ) error {
		differ.nilMapsAreEmpty = b
		return nil
	}
}

// WithNilPointersAreZero causes a nil pointer to be equal to a zero value.
func WithNilPointersAreZero(b bool) Opt {
	return func(differ *Differ) error {
		differ.nilPointersAreZero = b
		return nil
	}
}

// WithIgnoreSliceOrder causes Equal to ignore slice order so that
// []int{1, 2} and []int{2, 1} are equal. Only slices of primitive scalars
// like numbers and strings are supported. Slices of complex types,
// like []T where T is a struct, are undefined because Equal does not
// recurse into the slice value when this flag is enabled.
func WithIgnoreSliceOrder(b bool) Opt {
	return func(differ *Differ) error {
		differ.ignoreSliceOrder = b
		return nil
	}
}

type Differ struct {
	compareFunctions        bool
	compareUnexportedFields bool
	floatPrecision          int
	ignoreSliceOrder        bool
	logErrors               bool
	maxDepth                int
	maxDiff                 int
	nilMapsAreEmpty         bool
	nilPointersAreZero      bool
	nilSlicesAreEmpty       bool
}

func New(opts ...Opt) (d Differ, err error) {
	d = Differ{
		// options where zero-value equals default value are omitted
		floatPrecision: 10,
		maxDiff:        10,
	}
	for opt := range opts {
		err = opts[opt](&d)
		if err != nil {
			return d, fmt.Errorf("invalid option: %w", err)
		}
	}
	return d, nil
}

type Delta []string

func (d Delta) Equal(other Delta) bool {
	if len(d) != len(other) {
		return false
	}
	for i := range d {
		if d[i] != other[i] {
			return false
		}
	}
	return true
}

func (d Delta) ToSlice() []string {
	return d
}

// Compare compares variables a and b, recursing into their structure up to
// MaxDepth levels deep (if greater than zero), and returns a list of differences,
// or nil if there are none. Some differences may not be found if an error is
// also returned.
//
// If a type has an Equal method, like time.Equal, it is called to check for
// equality.
//
// When comparing a struct, if a field has the tag `deep:"-"` then it will be
// ignored.
func (d Differ) Compare(a, b any) Delta {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)
	c := &cmp{
		conf:        d,
		diff:        []string{},
		buff:        []string{},
		floatFormat: fmt.Sprintf("%%.%df", d.floatPrecision),
	}
	if a == nil && b == nil {
		return nil
	} else if a == nil && b != nil {
		c.saveDiff("<nil pointer>", b)
	} else if a != nil && b == nil {
		c.saveDiff(a, "<nil pointer>")
	}
	if len(c.diff) > 0 {
		return c.diff
	}

	c.equals(aVal, bVal, 0)
	if len(c.diff) > 0 {
		return c.diff // diffs
	}
	return nil // no diffs
}
