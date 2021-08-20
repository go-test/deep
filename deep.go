// Package deep provides function deep.Equal which is like reflect.DeepEqual but returns a list of differences.
// This is helpful when comparing complex types like structures and maps.
package deep

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"
)

var (
	// FloatPrecision is the number of decimal places to round float values
	// to when comparing.
	FloatPrecision = 10

	// TimePrecision is a precision used for time.Time.Truncate(), if it is non-zero.
	TimePrecision time.Duration

	// MaxDiff specifies the maximum number of differences to return.
	MaxDiff = 10

	// MaxDepth specifies the maximum levels of a struct to recurse into,
	// if greater than zero. If zero, there is no limit.
	MaxDepth = 0

	// LogErrors causes errors to be logged to STDERR when true.
	LogErrors = false

	// CompareUnexportedFields causes unexported struct fields, like s in
	// T{s int}, to be compared when true.
	CompareUnexportedFields = false

	// CompareFunctions causes functions to be compared according to reflect.DeepEqual rules:
	// that is, Func values are equal if both are nil; otherwise they are not equal.
	CompareFunctions = false

	// NilSlicesAreEmpty causes a nil slice to be equal to an empty slice.
	NilSlicesAreEmpty = false

	// NilMapsAreEmpty causes a nil map to be equal to an empty map.
	NilMapsAreEmpty = false
)

var (
	// ErrMaxRecursion is logged when MaxDepth is reached.
	ErrMaxRecursion = errors.New("recursed to MaxDepth")

	// ErrTypeMismatch is logged when Equal passed two different types of values.
	ErrTypeMismatch = errors.New("variables are different reflect.Type")

	// ErrNotHandled is logged when a primitive Go kind is not handled.
	ErrNotHandled = errors.New("cannot compare the reflect.Kind")
)

type cmp struct {
	diff []string
	buff []string
	seen map[uintptr]struct{}

	floatFormat string
}

var (
	errorType    = reflect.TypeOf((*error)(nil)).Elem()
	timeType     = reflect.TypeOf(time.Time{})
	durationType = reflect.TypeOf(time.Nanosecond)
)

// Equal compares variables a and b, recursing into their structure up to
// MaxDepth levels deep (if greater than zero), and returns a list of differences,
// or nil if there are none. Some differences may not be found if an error is
// also returned.
//
// If a type has an Equal method, like time.Equal, it is called to check for
// equality.
//
// When comparing a struct, if a field has the tag `deep:"-"` then it will be
// ignored.
func Equal(a, b interface{}) []string {
	c := &cmp{
		seen: make(map[uintptr]struct{}),

		floatFormat: fmt.Sprintf("%%.%df", FloatPrecision),
	}

	if a == nil || b == nil {
		switch {
		case b != nil:
			c.saveDiff("<untyped nil>", b)

		case a != nil:
			c.saveDiff(a, "<untyped nil>")
		}

		return c.diff
	}

	c.equals(reflect.ValueOf(a), reflect.ValueOf(b), 0)

	return c.diff
}

func (c *cmp) equals(a, b reflect.Value, level int) {
	if len(c.diff) >= MaxDiff {
		return
	}

	if MaxDepth > 0 && level > MaxDepth {
		logError(ErrMaxRecursion)
		return
	}

	// Check if one value is nil, e.g. T{x: *X} and T.x is nil
	if !a.IsValid() || !b.IsValid() {
		switch {
		case a.IsValid():
			c.saveDiff(a.Type(), "<invalid value>")

		case b.IsValid():
			c.saveDiff("<invalid value>", b.Type())
		}

		return
	}

	// If different types, they can't be equal
	aType := a.Type()
	bType := b.Type()
	if aType != bType {
		logError(ErrTypeMismatch)

		// Built-in types don't have a name, so don't report [3]int != [2]int as " != "
		if aType.Name() == "" || aType.Name() != bType.Name() {
			c.saveDiff(aType, bType)
			return
		}

		// Type names can be the same, e.g. pkg/v1.Error and pkg/v2.Error
		// are both exported as pkg, so unless we include the full pkg path
		// the diff will be "pkg.Error != pkg.Error"
		// https://github.com/go-test/deep/issues/39
		aFullType := aType.PkgPath() + "." + aType.Name()
		bFullType := bType.PkgPath() + "." + bType.Name()

		c.saveDiff(aFullType, bFullType)
		return
	}

	// Primitive https://golang.org/pkg/reflect/#Kind
	kind := a.Kind() // We know aType == bType, so a.Kind() == b.Kind()

	// Do a and b have underlying elements? Yes, if they're ptr or interface.
	elem := kind == reflect.Ptr || kind == reflect.Interface

	// If both types implement the error interface, compare the error strings.
	// This must be done before dereferencing because the interface may be on a pointer receiver.
	// Re https://github.com/go-test/deep/issues/31, a/b might be primitive kinds; see TestErrorPrimitiveKind.
	if aType.Implements(errorType) {
		if !elem || (!a.IsNil() && !b.IsNil()) {
			aFunc := a.MethodByName("Error")
			bFunc := b.MethodByName("Error")

			if aFunc.CanInterface() && bFunc.CanInterface() {
				aString := aFunc.Call(nil)[0].String()
				bString := bFunc.Call(nil)[0].String()
				if aString != bString {
					c.saveDiff(aString, bString)
				}
				return
			}
		}
	}

	if TimePrecision > 0 {
		switch aType {
		case timeType, durationType:
			aFunc := a.MethodByName("Truncate")
			bFunc := a.MethodByName("Truncate")

			if aFunc.CanInterface() && bFunc.CanInterface() {
				precision := reflect.ValueOf(TimePrecision)

				a = aFunc.Call([]reflect.Value{precision})[0]
				b = bFunc.Call([]reflect.Value{precision})[0]
			}
		}
	}

	// For types with an `Equal(bType) bool` method like time.Time, we want to use that.
	// But not if it is from an unexported struct field (CanInterface).
	if eqFunc := a.MethodByName("Equal"); eqFunc.IsValid() && eqFunc.CanInterface() {
		// Handle https://github.com/go-test/deep/issues/15:
		// Don't call a.Equal if the method is from an embedded struct, like:
		//   type Foo struct { time.Time }
		// First, we'll encounter Equal(Foo, time.Time),
		// but if we pass b as the 2nd argument, then we'll panic: "Call using pkg.Foo as type time.Time"
		// As far as I can tell, there's no way to see that the method is from time.Time not Foo.
		// So we check the type of the 1st (0) arg and skip unless it's b type.
		// Later, we'll encounter the time.Time anonymous/embedded field,
		// and then we'll have Equal(time.Time, time.Time).
		typ := eqFunc.Type()
		switch {
		case typ.NumIn() != 1, typ.In(0) != bType:
			// Equal does not take one argument of the same type.
		case typ.NumOut() != 1, typ.Out(0).Kind() != reflect.Bool:
			// Equal does not return only one value of kind bool.
		default:
			retVals := eqFunc.Call([]reflect.Value{b})
			if !retVals[0].Bool() {
				c.saveDiff(a, b)
			}
			return
		}
	}

	// Dereference pointers and interfaces
	if elem {
		if a.IsNil() || b.IsNil() {
			if !a.IsNil() {
				for a.Kind() == reflect.Interface {
					// resolve a to its concrete value.
					a = a.Elem()
				}
				c.saveDiff(a.Type(), "<nil pointer>")
			}

			if !b.IsNil() {
				for b.Kind() == reflect.Interface {
					// resolve b to its concrete value.
					b = b.Elem()
				}
				c.saveDiff("<nil pointer>", b.Type())
			}

			return
		}

		if kind == reflect.Ptr {
			if c.haveSeen(a.Pointer(), b.Pointer()) {
				return
			}

			c.saw(a.Pointer(), b.Pointer())
		}

		c.equals(a.Elem(), b.Elem(), level+1)
		return
	}

	switch kind {

	/////////////////////////////////////////////////////////////////////
	// Iterable kinds
	/////////////////////////////////////////////////////////////////////

	case reflect.Struct:
		/*
			The variables are structs like:
				type T struct {
					FirstName string
					LastName  string
				}
			Type = <pkg>.T, Kind = reflect.Struct

			Iterate through the fields (FirstName, LastName), recurse into their values.
		*/

		for i := 0; i < a.NumField(); i++ {
			if len(c.diff) >= MaxDiff {
				return
			}

			if aType.Field(i).PkgPath != "" && !CompareUnexportedFields {
				continue // skip unexported field, e.g. s in type T struct {s string}
			}

			if aType.Field(i).Tag.Get("deep") == "-" {
				continue // field wants to be ignored
			}

			c.push(aType.Field(i).Name)
			c.equals(a.Field(i), b.Field(i), level+1)
			c.pop()
		}

	case reflect.Map:
		/*
			The variables are maps like:
				map[string]int{
					"foo": 1,
					"bar": 2,
				}
			Type = map[string]int, Kind = reflect.Map

			Or:
				type T map[string]int{}
			Type = <pkg>.T, Kind = reflect.Map

			Iterate through the map keys (foo, bar), recurse into their values.
		*/

		if a.IsNil() || b.IsNil() {
			if NilMapsAreEmpty {
				if b.Len() != 0 {
					c.saveDiff("<nil map>", b)
				}

				if a.Len() != 0 {
					c.saveDiff(a, "<nil map>")
				}

				return
			}

			if !b.IsNil() {
				c.saveDiff("<nil map>", b)
			}

			if !a.IsNil() {
				c.saveDiff(a, "<nil map>")
			}

			return
		}

		if a.Pointer() == b.Pointer() {
			return
		}

		prefix := func(key reflect.Value) string { return fmt.Sprintf("map[%v]", key) }

		for _, key := range a.MapKeys() {
			if len(c.diff) >= MaxDiff {
				return
			}

			aVal := a.MapIndex(key)
			bVal := b.MapIndex(key)

			if !bVal.IsValid() {
				c.prefixDiff(prefix(key), aVal, "<does not have key>")
				continue
			}

			c.push(prefix(key))
			c.equals(aVal, bVal, level+1)
			c.pop()
		}

		for _, key := range b.MapKeys() {
			if len(c.diff) >= MaxDiff {
				return
			}

			if aVal := a.MapIndex(key); aVal.IsValid() {
				continue
			}

			c.prefixDiff(prefix(key), "<does not have key>", b.MapIndex(key))
		}

	case reflect.Array:
		n := a.Len()
		for i := 0; i < n; i++ {
			if len(c.diff) >= MaxDiff {
				return
			}

			c.push(fmt.Sprintf("array[%d]", i))
			c.equals(a.Index(i), b.Index(i), level+1)
			c.pop()
		}

	case reflect.Slice:
		if a.IsNil() || b.IsNil() {
			if NilSlicesAreEmpty {
				if b.Len() != 0 {
					c.saveDiff("<nil slice>", b)
				}

				if a.Len() != 0 {
					c.saveDiff(a, "<nil slice>")
				}

				return
			}

			if !b.IsNil() {
				c.saveDiff("<nil slice>", b)
			}
			if !a.IsNil() {
				c.saveDiff(a, "<nil slice>")
			}

			return
		}

		aLen := a.Len()
		bLen := b.Len()

		prefix := func(i int) string { return fmt.Sprintf("slice[%d]", i) }

		if a.Pointer() != b.Pointer() {
			// These values can only be different if they have different backing store arrays.
			// So, there is no need to check them if a.Pointer() == b.Pointer().

			n := aLen
			if n > bLen {
				n = bLen
			}

			for i := 0; i < n; i++ {
				if len(c.diff) >= MaxDiff {
					return
				}

				c.push(prefix(i))
				c.equals(a.Index(i), b.Index(i), level+1)
				c.pop()
			}
		}

		for i := bLen; i < aLen; i++ {
			if len(c.diff) >= MaxDiff {
				return
			}

			c.prefixDiff(prefix(i), a.Index(i), "<no value>")
		}

		for i := aLen; i < bLen; i++ {
			if len(c.diff) >= MaxDiff {
				return
			}

			c.prefixDiff(prefix(i), "<no value>", b.Index(i))
		}

	/////////////////////////////////////////////////////////////////////
	// Primitive kinds
	/////////////////////////////////////////////////////////////////////

	case reflect.Float32, reflect.Float64:
		// Zero and negative-zero format to different strings.
		// The equality test here short-circuits all cases where values are equal by definition.
		// Strictly, this test is only necessary for the case of zero and negative-zero,
		// but this actual-equality short-circuit is useful for all cases.
		if a.Float() == b.Float() {
			return
		}

		// Round floats to FloatPrecision decimal places to compare with user-defined precision.
		// As is commonly known, floats have "imprecision" such that 0.1 becomes 0.100000001490116119384765625.
		// This cannot be avoided; it can only be handled.
		// Issue 30 suggested that floats be compared using an epsilon: equal = |a-b| < epsilon.
		// In many cases the result is the same,
		// but I think epsilon is a little less clear for users to reason about.
		// See issue 30 for details.

		aval := fmt.Sprintf(c.floatFormat, a.Float())
		bval := fmt.Sprintf(c.floatFormat, b.Float())
		if aval != bval {
			c.saveDiff(a, b)
		}

	case reflect.Bool:
		if a.Bool() != b.Bool() {
			c.saveDiff(a, b)
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			c.saveDiff(a, b)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Uint() != b.Uint() {
			c.saveDiff(a, b)
		}

	case reflect.String:
		if a.String() != b.String() {
			c.saveDiff(a, b)
		}

	/////////////////////////////////////////////////////////////////////
	// Edge-cases
	/////////////////////////////////////////////////////////////////////

	case reflect.Func:
		if CompareFunctions {
			if a.IsNil() || b.IsNil() {
				if !a.IsNil() {
					c.saveDiff("<non-nil func>", "<nil func>")
				}

				if !b.IsNil() {
					c.saveDiff("<nil func>", "<non-nil func>")
				}

				return
			}

			c.saveDiff("<non-nil func>", "<non-nil func>")
		}

	default:
		logError(ErrNotHandled)
	}
}

func (c *cmp) saw(ptrs ...uintptr) {
	for _, ptr := range ptrs {
		c.seen[ptr] = struct{}{}
	}
}

func (c *cmp) haveSeen(ptrs ...uintptr) bool {
	for _, ptr := range ptrs {
		if _, ok := c.seen[ptr]; ok {
			return true
		}
	}

	return false
}

func (c *cmp) push(name string) {
	c.buff = append(c.buff, name)
}

func (c *cmp) pop() {
	if len(c.buff) > 0 {
		c.buff = c.buff[0 : len(c.buff)-1]
	}
}

func formatDiff(prefixes []string, aval, bval interface{}) string {
	if len(prefixes) > 0 {
		prefix := strings.Join(prefixes, ".")
		return fmt.Sprintf("%s: %v != %v", prefix, aval, bval)
	}

	return fmt.Sprintf("%v != %v", aval, bval)
}

func (c *cmp) saveDiff(aval, bval interface{}) {
	c.diff = append(c.diff, formatDiff(c.buff, aval, bval))
}

func (c *cmp) prefixDiff(prefix string, aval, bval interface{}) {
	c.diff = append(c.diff, formatDiff(append(c.buff, prefix), aval, bval))
}

func logError(err error) {
	if LogErrors {
		log.Println(err)
	}
}
