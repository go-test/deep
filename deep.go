// Package deep provides function deep.Equal which is like reflect.DeepEqual but
// returns a list of differences. This is helpful when comparing complex types
// like structures and maps.
package deep

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"regexp"
)

var (
	// FloatPrecision is the number of decimal places to round float values
	// to when comparing.
	FloatPrecision = 10

	// MaxDiff specifies the maximum number of differences to return.
	MaxDiff = 10000

	// MaxDepth specifies the maximum levels of a struct to recurse into,
	// if greater than zero. If zero, there is no limit.
	MaxDepth = 0

	// LogErrors causes errors to be logged to STDERR when true.
	LogErrors = false

	// CompareUnexportedFields causes unexported struct fields, like s in
	// T{s int}, to be comparsed when true.
	CompareUnexportedFields = false
)

var (
	// ErrMaxRecursion is logged when MaxDepth is reached.
	ErrMaxRecursion = errors.New("recursed to MaxDepth")

	// ErrTypeMismatch is logged when Equal passed two different types of values.
	ErrTypeMismatch = errors.New("variables are different reflect.Type")

	// ErrNotHandled is logged when a primitive Go kind is not handled.
	ErrNotHandled = errors.New("cannot compare the reflect.Kind")
)

//this is used for our json comparison. key = unique value, left = index of left object, right = index of right object.
type arrayCmp struct {
	key 	string
	left	int
	right	int
}

type cmp struct {
	diff        []string
	buff        []string
	floatFormat string
	callback    func(ta reflect.Type, tb reflect.Type, va interface{}, vb interface{}, text string) bool
	callback2	func(field string) []string
	callback3	func(log string, end bool)
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Compare compares variables a and b, recursing into their structure up to
// MaxDepth levels deep (if greater than zero), and returns a list of differences,
// or nil if there are none. Some differences may not be found if an error is
// also returned.
//
// If a type has an Equal method, like time.Equal, it is called to check for
// equality.
func Compare(a interface{}, b interface{},
	callback func(ta reflect.Type, tb reflect.Type, va interface{}, vb interface{}, text string) bool,
	callback2 func(field string) []string,
	callback3 func(log string, end bool)) []string {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)
	c := &cmp{
		diff:        []string{},
		buff:        []string{},
		floatFormat: fmt.Sprintf("%%.%df", FloatPrecision),
		callback:    callback,
		callback2:	 callback2,
		callback3:	 callback3,
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

// Equal compares variables a and b, recursing into their structure up to
// MaxDepth levels deep (if greater than zero), and returns a list of differences,
// or nil if there are none. Some differences may not be found if an error is
// also returned.
//
// If a type has an Equal method, like time.Equal, it is called to check for
// equality.
func Equal(a interface{}, b interface{}) []string {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)
	c := &cmp{
		diff:        []string{},
		buff:        []string{},
		floatFormat: fmt.Sprintf("%%.%df", FloatPrecision),
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

func (c *cmp) equals(a, b reflect.Value, level int) {
	if MaxDepth > 0 && level > MaxDepth {
		logError(ErrMaxRecursion)
		return
	}

	// Check if one value is nil, e.g. T{x: *X} and T.x is nil
	if !a.IsValid() || !b.IsValid() {
		if a.IsValid() && !b.IsValid() {
			c.saveDiff(a.Type(), "<nil pointer>")
		} else if !a.IsValid() && b.IsValid() {
			c.saveDiff("<nil pointer>", b.Type())
		}
		return
	}

	// If differenet types, they can't be equal
	aType := a.Type()
	bType := b.Type()
	if aType != bType {
		c.saveDiff(aType, bType)
		logError(ErrTypeMismatch)
		return
	}

	// Primitive https://golang.org/pkg/reflect/#Kind
	aKind := a.Kind()
	bKind := b.Kind()

	// If both types implement the error interface, compare the error strings.
	// This must be done before dereferencing because the interface is on a
	// pointer receiver.
	if aType.Implements(errorType) && bType.Implements(errorType) {
		if a.Elem().IsValid() && b.Elem().IsValid() { // both err != nil
			aString := a.MethodByName("Error").Call(nil)[0].String()
			bString := b.MethodByName("Error").Call(nil)[0].String()
			if aString != bString {
				c.saveDiff(aString, bString)
			}
			return
		}
	}

	// Dereference pointers and interface{}
	if aElem, bElem := (aKind == reflect.Ptr || aKind == reflect.Interface),
		(bKind == reflect.Ptr || bKind == reflect.Interface); aElem || bElem {

		if aElem {
			a = a.Elem()
		}

		if bElem {
			b = b.Elem()
		}

		c.equals(a, b, level+1)
		return
	}

	switch aKind {

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

		// Types with an Equal() method, like time.Time, only if struct field
		// is exported (CanInterface)
		if eqFunc := a.MethodByName("Equal"); eqFunc.IsValid() && eqFunc.CanInterface() {
			// Handle https://github.com/go-test/deep/issues/15:
			// Don't call T.Equal if the method is from an embedded struct, like:
			//   type Foo struct { time.Time }
			// First, we'll encounter Equal(Ttime, time.Time) but if we pass b
			// as the 2nd arg we'll panic: "Call using pkg.Foo as type time.Time"
			// As far as I can tell, there's no way to see that the method is from
			// time.Time not Foo. So we check the type of the 1st (0) arg and skip
			// unless it's b type. Later, we'll encounter the time.Time anonymous/
			// embedded field and then we'll have Equal(time.Time, time.Time).
			funcType := eqFunc.Type()
			if funcType.NumIn() == 1 && funcType.In(0) == bType {
				retVals := eqFunc.Call([]reflect.Value{b})
				if !retVals[0].Bool() {
					c.saveDiff(a, b)
				}
				return
			}
		}

		for i := 0; i < a.NumField(); i++ {
			if aType.Field(i).PkgPath != "" && !CompareUnexportedFields {
				continue // skip unexported field, e.g. s in type T struct {s string}
			}

			c.push(aType.Field(i).Name) // push field name to buff

			// Get the Value for each field, e.g. FirstName has Type = string,
			// Kind = reflect.String.
			af := a.Field(i)
			bf := b.Field(i)

			// Recurse to compare the field values
			c.equals(af, bf, level+1)

			c.pop() // pop field name from buff

			if len(c.diff) >= MaxDiff {
				break
			}
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
			if a.IsNil() && !b.IsNil() {
				c.saveDiff("<nil map>", b)
			} else if !a.IsNil() && b.IsNil() {
				c.saveDiff(a, "<nil map>")
			}
			return
		}

		if a.Pointer() == b.Pointer() {
			return
		}

		for _, key := range a.MapKeys() {
			c.push(fmt.Sprintf("map[%s]", key))

			aVal := a.MapIndex(key)
			bVal := b.MapIndex(key)
			if bVal.IsValid() {
				c.equals(aVal, bVal, level+1)
			} else {
				c.saveDiff(aVal, "<does not have key>")
			}

			c.pop()

			if len(c.diff) >= MaxDiff {
				return
			}
		}

		for _, key := range b.MapKeys() {
			if aVal := a.MapIndex(key); aVal.IsValid() {
				continue
			}

			c.push(fmt.Sprintf("map[%s]", key))
			c.saveDiff("<does not have key>", b.MapIndex(key))
			c.pop()
			if len(c.diff) >= MaxDiff {
				return
			}
		}
	case reflect.Array:
		n := a.Len()
		for i := 0; i < n; i++ {
			c.push(fmt.Sprintf("array[%d]", i))
			c.equals(a.Index(i), b.Index(i), level+1)
			c.pop()
			if len(c.diff) >= MaxDiff {
				break
			}
		}
	case reflect.Slice:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() && !b.IsNil() {
				c.saveDiff("<nil slice>", b)
			} else if !a.IsNil() && b.IsNil() {
				c.saveDiff(a, "<nil slice>")
			}
			return
		}

		if a.Pointer() == b.Pointer() {
			return
		}

		if c.callback2 != nil {
			//before we move further, we build our cmap and run c.equals on the correct parts.
			var (
				key string
			)
			if c.buff != nil {
				var rgx = regexp.MustCompile(`map\[(.*?)]`)
				var rs []string
				for i := len(c.buff)-1; i >= 0; i-- {
					if rs = rgx.FindStringSubmatch(c.buff[i]); rs != nil {
						key = rs[1]
					}
				}				
			}

			//see if we can grab our unique key(s)
			if unique := c.callback2(fmt.Sprintf("%s", key)); unique != nil {
				var cmap map[string]arrayCmp
				cmap = make(map[string]arrayCmp)
				aLen := a.Len()
				bLen := b.Len()
				n := aLen

				//first part - iterate over a and fill up cmap. Everything we see here is going to go into cmap with it's unique value as the key
				//and the cmp structure as a value. The cmp structure will have left filled in as the current index of a.
				for i := 0; i < n; i++ {
					var field 	reflect.Value
					var val 	string
					obj, ok := getObj(a, i)
					if !ok {
						err := fmt.Sprintf("Type error. Files may be different or type is unrecognized. Ending compare.")
						c.callback3(err, true)
						return
					}
					//get key == unique
					//build our uniquely named key. unique will be something like ["service_name", "user"]. we will look for each of those fieldnames inside of a[i]
					//and add the values of those fields to val. ie an object with service_name: cron and user:devtest, our key for cmap will be crondevtest.
					//TONOTE: order for these loops MATTER. MapKeys() will return keys out of order, which will flip our key name when we don't want it to.
					//we force it to be in order by looping through our list first.
					for _, fieldName := range unique {
						for _, key := range obj.MapKeys() {
					
						test := (fmt.Sprintf("%s", key))

							if fieldName == test {
									field = key
									val = val + fmt.Sprintf("%s", obj.MapIndex(field))
									break
							}

						}
					}

					comp := &arrayCmp{
						key: 	val,
						left:	i,
						right:	-1,
					}
					cmap[val] = *comp

					
				}

				//loop through b. for each elem in b, check if it's in cmap. if it is, update 'right' in the structure with the correct index. if it is not in cmap,
				//add it with left = -1. 
				n = bLen
				for i := 0; i < n; i++ {
					var field 	reflect.Value
					var val 	string
					obj, ok := getObj(b, i)
					if !ok {
						err := fmt.Sprintf("Type error. Files may be different or type is unrecognized. Ending compare.")
						c.callback3(err, true)
						return
					}
					//get key == unique
					for _, fieldName := range unique {
						for _, key := range obj.MapKeys() {
			
						test := (fmt.Sprintf("%s", key))

							if fieldName == test {
									field = key
									val = val + fmt.Sprintf("%s", obj.MapIndex(field))
									break
							}

						}
					}

					//look inside of cmap for val. if there, add right index to struct. else, make new
					if comp, ok := cmap[val]; ok {
						comp.right = i
						cmap[val] = comp
					} else {
						comp := &arrayCmp{
							key: 	val,
							left:	-1,
							right:	i,
						}
						cmap[val] = *comp
					}					
				}

				//loop through cmap. decide what gets compared and what is confirmed different. we call .equals here on the correct indexes
				for _, comp := range cmap {
					if comp.left != -1 && comp.right != -1 {
						c.push(fmt.Sprintf("slice[%d]", comp.left))
						c.equals(a.Index(comp.left), b.Index(comp.right), level+1)
					} else if comp.left != -1 {
						c.push(fmt.Sprintf("slice[%d]", comp.left))
						c.saveDiff(a.Index(comp.left), "<no value>")
					} else {
						c.push(fmt.Sprintf("slice[%d]", comp.right))
						c.saveDiff("<no value>", b.Index(comp.right))
					}
					c.pop()
					if len(c.diff) >= MaxDiff {
						break
					}
				}
				break
			}

		}

		aLen := a.Len()
		bLen := b.Len()
		n := aLen
		if bLen > aLen {
			n = bLen
		}
		for i := 0; i < n; i++ {
			c.push(fmt.Sprintf("slice[%d]", i))
			if i < aLen && i < bLen {
				c.equals(a.Index(i), b.Index(i), level+1)
			} else if i < aLen {
				c.saveDiff(a.Index(i), "<no value>")
			} else {
				c.saveDiff("<no value>", b.Index(i))
			}
			c.pop()
			if len(c.diff) >= MaxDiff {
				break
			}
		}

	/////////////////////////////////////////////////////////////////////
	// Primitive kinds
	/////////////////////////////////////////////////////////////////////

	case reflect.Float32, reflect.Float64:
		// Avoid 0.04147685731961082 != 0.041476857319611
		// 6 decimal places is close enough
		aval := fmt.Sprintf(c.floatFormat, a.Float())
		bval := fmt.Sprintf(c.floatFormat, b.Float())
		if aval != bval {
			c.saveDiff(a.Float(), b.Float())
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			c.saveDiff(a.Bool(), b.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			c.saveDiff(a.Int(), b.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Uint() != b.Uint() {
			c.saveDiff(a.Uint(), b.Uint())
		}
	case reflect.String:
		if a.String() != b.String() {
			c.saveDiff(a.String(), b.String())
		}

	default:
		logError(ErrNotHandled)
	}
}

func (c *cmp) push(name string) {
	c.buff = append(c.buff, name)
}

func (c *cmp) pop() {
	if len(c.buff) > 0 {
		c.buff = c.buff[0 : len(c.buff)-1]
	}
}

//this was also heavily modified. we shape the output so it is even more readable for humans and we know exactly what objects are there or not.
func (c *cmp) saveDiff(aval, bval interface{}) {
	var diff string
	var varName string
	var rgx = regexp.MustCompile(`map\[(.*?)]`)
	if len(c.buff) > 0 {
		varName = strings.Join(c.buff, ".")
		aString := fmt.Sprintf("%v", aval)
		bString := fmt.Sprintf("%v", bval)
		//<no value> is the case where both fields exist, but an item(s) isn't there. ie 3 objects in services vs 5 objects in services.
		if aString == "<no value>" || bString == "<no value>" {
			n := len(c.buff)
			for i := n-1; i >= 0; i-- {
				if rx := rgx.FindStringSubmatch(c.buff[i]); rx != nil {
					key := rx[1]
					var m reflect.Value
					var output string
					//we use our callback to find out what is important to this key. this is how we'll tell the user what is missing
					unique := c.callback2(key)
					switch aval.(type) {
					case reflect.Value:
						m = aval.(reflect.Value).Elem()
					default:
						m = bval.(reflect.Value).Elem()
					}
					if m.Kind() == reflect.Map {
						for i, _ := range unique {
							for _, key := range m.MapKeys() {
								tmp := fmt.Sprintf("%s", key)
								if unique[i] == tmp {
									val := fmt.Sprintf("%v", m.MapIndex(key))
									output = output + fmt.Sprintf("%s:%s ", unique[i], val)
									break
								}
							}
						}
					} else {
						diff = fmt.Sprintf("%s: %v != %v", varName, aval, bval)
						c.callback3("Type Error. Difference " + diff + "not saved.", true)
					}
					if output == "" {
						output = output + fmt.Sprintf("%v", m)
					}
					if aString == "<no value>" {
						diff = fmt.Sprintf("%s: New item in %s with unique field(s): %s", varName, key, output)
					} else {
						diff = fmt.Sprintf("%s: Missing item in %s with unique field(s): %s", varName, key, output)
					}
					break
				}
			}
		//<does not have key> is seen when one side is missing an entire field. ie 'a' has 'services' but 'b' has no 'services' field at all.
		} else if aString == "<does not have key>" || bString == "<does not have key>" {
			var output string
			for i, key := range c.buff {
				//we want to explain exactly what's missing; this will tell the user that the section monitoring_info -> nics is missing.
				if rx := rgx.FindStringSubmatch(key); rx != nil {
					if i != 0 {
						output = output + fmt.Sprintf(" -> ")
					}
					output = output + fmt.Sprintf(rx[1])
				}
			}
			if aString == "does not have key>" {
				diff = fmt.Sprintf("%s: New field with data: %s", varName, output)
			} else {
				diff = fmt.Sprintf("%s: Missing field: %s", varName, output)
			}
		} else {
			diff = fmt.Sprintf("%s: %v != %v", varName, aval, bval)
		}
	} else {

		diff = fmt.Sprintf("%v != %v", aval, bval)
		c.diff = append(c.diff, diff)
	}

	addDiff := true
	if c.callback != nil {
		aType := reflect.TypeOf(aval)
		bType := reflect.TypeOf(bval)
		addDiff = c.callback(aType, bType, aval, bval, diff)
	}

	if addDiff {
		c.diff = append(c.diff, diff)
	}

}

func logError(err error) {
	if LogErrors {
		log.Println(err)
	}
}

func getObj(a reflect.Value, i int) (obj reflect.Value, ok bool) {
	obj = a.Index(i).Elem()
	defer func() {
		if r := recover(); r != nil {
			//we panicked because of types
			ok = false
		} else {
			ok = obj.Kind() == reflect.Map
		}
	}()
	return obj, ok
}