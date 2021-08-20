package deep_test

import (
	"errors"
	"fmt"
	"math"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/go-test/deep"
	v1 "github.com/go-test/deep/test/v1"
	v2 "github.com/go-test/deep/test/v2"
)

func shouldBeEqual(t testing.TB, diff []string) {
	t.Helper()

	if len(diff) > 0 {
		t.Errorf("should be equal: %q", diff)
	}
}

func shouldBeMaxDiff(t testing.TB, diff []string) {
	t.Helper()

	if len(diff) == 0 {
		t.Fatal("no diffs")
	}

	if len(diff) != deep.MaxDiff {
		t.Logf("diff: %q", diff)
		t.Errorf("wrong number of diffs: got %d, expected %d", len(diff), deep.MaxDiff)
	}
}

const (
	multilineTestError = `wrong diff:
	got:      %q
	expected: %q`
)

func reportWrongDiff(t testing.TB, got, expect string) {
	t.Helper()

	output := fmt.Sprintf("wrong diff: got %q, expected %q", got, expect)
	if len(output) > 120 {
		output = fmt.Sprintf(multilineTestError, got, expect)
	}

	t.Error(output)
}

func shouldBeDiffs(t testing.TB, diff []string, head string, tail ...string) {
	t.Helper()

	if len(diff) == 0 {
		t.Fatal("no diffs")
	}

	if len(diff) != len(tail)+1 {
		t.Log("diff:", diff)
		t.Errorf("wrong number of diffs: got %d, expected %d", len(diff), len(tail)+1)
	}

	if expect := head; diff[0] != expect {
		reportWrongDiff(t, diff[0], expect)
	}

	for i, expect := range tail {
		if i+1 >= len(diff) {
			t.Errorf("missing diff: %q", expect)
			continue
		}

		if got := diff[i+1]; got != expect {
			reportWrongDiff(t, got, expect)
		}

	}
}

func TestString(t *testing.T) {
	shouldBeEqual(t, deep.Equal("foo", "foo"))

	shouldBeDiffs(t, deep.Equal("foo", "bar"), "foo != bar")
}

func TestFloat(t *testing.T) {
	shouldBeEqual(t, deep.Equal(1.1, 1.1))

	shouldBeDiffs(t, deep.Equal(1.1234561, 1.1234562), "1.1234561 != 1.1234562")

	shouldBeEqual(t, deep.Equal(float32(0.3), float32(0.1)+float32(0.2)))
	shouldBeEqual(t, deep.Equal(float64(0.3), float64(0.1)+float64(0.2)))

	restoreFloatPrecision := deep.FloatPrecision
	t.Cleanup(func() { deep.FloatPrecision = restoreFloatPrecision })

	deep.FloatPrecision = 6

	shouldBeEqual(t, deep.Equal(1.1234561, 1.1234562))

	shouldBeDiffs(t, deep.Equal(1.123456, 1.123457), "1.123456 != 1.123457")

	// Since we compare string representations, NaN should compare equal to NaN
	shouldBeEqual(t, deep.Equal(math.NaN(), math.NaN()))

	shouldBeEqual(t, deep.Equal(math.Inf(1), math.Inf(1)))
	shouldBeEqual(t, deep.Equal(math.Inf(-1), math.Inf(-1)))

	shouldBeDiffs(t, deep.Equal(math.Inf(1), math.Inf(-1)), "+Inf != -Inf")
	shouldBeDiffs(t, deep.Equal(math.Inf(-1), math.Inf(1)), "-Inf != +Inf")

	var zero float64

	shouldBeEqual(t, deep.Equal(zero, zero))
	shouldBeEqual(t, deep.Equal(-zero, -zero))

	shouldBeEqual(t, deep.Equal(zero, -zero))
	shouldBeEqual(t, deep.Equal(-zero, zero))
}

func TestInt(t *testing.T) {
	shouldBeEqual(t, deep.Equal(1, 1))

	shouldBeDiffs(t, deep.Equal(1, 2), "1 != 2")
}

func TestUint(t *testing.T) {
	shouldBeEqual(t, deep.Equal(uint(2), uint(2)))

	shouldBeDiffs(t, deep.Equal(uint(2), uint(3)), "2 != 3")
}

func TestBool(t *testing.T) {
	shouldBeEqual(t, deep.Equal(true, true))
	shouldBeEqual(t, deep.Equal(false, false))

	shouldBeDiffs(t, deep.Equal(true, false), "true != false")
}

func TestTypeMismatch(t *testing.T) {
	type T1 int // same type kind (int)
	type T2 int // but different type

	t1 := T1(1)
	t2 := T2(2)

	shouldBeDiffs(t, deep.Equal(t1, t2), "deep_test.T1 != deep_test.T2")

	// Same pkg name but differnet full paths
	// https://github.com/go-test/deep/issues/39
	err1 := v1.Error{}
	err2 := v2.Error{}
	shouldBeDiffs(t,
		deep.Equal(err1, err2),
		"github.com/go-test/deep/test/v1.Error != github.com/go-test/deep/test/v2.Error",
	)
}

func TestKindMismatch(t *testing.T) {
	restoreLogErrors := deep.LogErrors
	t.Cleanup(func() { deep.LogErrors = restoreLogErrors })

	deep.LogErrors = true

	shouldBeDiffs(t, deep.Equal(int(100), float64(100)), "int != float64")
}

func TestDeepRecursion(t *testing.T) {
	restoreMaxDepth := deep.MaxDepth
	t.Cleanup(func() { deep.MaxDepth = restoreMaxDepth })

	type (
		s1 struct {
			S int
		}
		s2 struct {
			S s1
		}
		s3 struct {
			S s2
		}
	)

	foo := map[string]s3{
		"foo": s3{ // 1
			S: s2{ // 2
				S: s1{ // 3
					S: 42, // 4
				},
			},
		},
	}
	bar := map[string]s3{
		"foo": s3{
			S: s2{
				S: s1{
					S: 100,
				},
			},
		},
	}

	// No diffs because MaxDepth=2 prevents seeing the diff at 3rd level down
	deep.MaxDepth = 2
	shouldBeEqual(t, deep.Equal(foo, bar))

	deep.MaxDepth = 4

	shouldBeDiffs(t, deep.Equal(foo, bar), "map[foo].S.S.S: 42 != 100")
}

func TestMaxDiff(t *testing.T) {
	restoreMaxDiff := deep.MaxDiff
	t.Cleanup(func() { deep.MaxDiff = restoreMaxDiff })

	deep.MaxDiff = 3

	a1 := []int{1, 2, 3, 4, 5, 6, 7}
	a2 := []int{0, 0, 0, 0, 0, 0, 0}

	shouldBeMaxDiff(t, deep.Equal(a1, a2))

	restoreCompareUnexportedFields := deep.CompareUnexportedFields
	t.Cleanup(func() { deep.CompareUnexportedFields = restoreCompareUnexportedFields })

	deep.CompareUnexportedFields = true

	type fiveFields struct {
		a int // unexported fields require: deep.CompareUnexportedFields = true
		b int
		c int
		d int
		e int
	}

	s1 := fiveFields{1, 2, 3, 4, 5}
	s2 := fiveFields{0, 0, 0, 0, 0}

	shouldBeMaxDiff(t, deep.Equal(s1, s2))

	// Same keys, too many diffs
	m1 := map[int]int{
		1: 1,
		2: 2,
		3: 3,
		4: 4,
		5: 5,
	}
	m2 := map[int]int{
		1: 0,
		2: 0,
		3: 0,
		4: 0,
		5: 0,
	}

	shouldBeMaxDiff(t, deep.Equal(m1, m2))

	// Too many missing keys
	m1 = map[int]int{
		1: 1,
		2: 2,
	}
	m2 = map[int]int{
		1: 1,
		2: 2,
		3: 0,
		4: 0,
		5: 0,
		6: 0,
		7: 0,
	}

	shouldBeMaxDiff(t, deep.Equal(m1, m2))
}

func TestNotHandled(t *testing.T) {
	shouldBeEqual(t, deep.Equal(func(int) {}, func(int) {}))
}

func TestStruct(t *testing.T) {
	type s struct {
		id     int
		Name   string
		Number int
	}

	s1 := s{
		id:     1,
		Name:   "foo",
		Number: 2,
	}
	s2 := s1

	shouldBeEqual(t, deep.Equal(s1, s2))

	s2.Name = "bar"
	shouldBeDiffs(t, deep.Equal(s1, s2), "Name: foo != bar")

	s2.Number = 22
	shouldBeDiffs(t, deep.Equal(s1, s2),
		"Name: foo != bar",
		"Number: 2 != 22",
	)

	s2.id = 11
	shouldBeDiffs(t, deep.Equal(s1, s2),
		"Name: foo != bar",
		"Number: 2 != 22",
		// should skip unexported fields
	)
}

func TestStructWithTags(t *testing.T) {
	type s1 struct {
		same                    int
		modified                int
		sameIgnored             int `deep:"-"`
		modifiedIgnored         int `deep:"-"`
		ExportedSame            int
		ExportedModified        int
		ExportedSameIgnored     int `deep:"-"`
		ExportedModifiedIgnored int `deep:"-"`
	}
	type s2 struct {
		s1
		same                    int
		modified                int
		sameIgnored             int `deep:"-"`
		modifiedIgnored         int `deep:"-"`
		ExportedSame            int
		ExportedModified        int
		ExportedSameIgnored     int `deep:"-"`
		ExportedModifiedIgnored int `deep:"-"`
		recurseInline           s1
		recursePtr              *s2
	}
	sa := s2{
		s1: s1{
			same:                    0,
			modified:                1,
			sameIgnored:             2,
			modifiedIgnored:         3,
			ExportedSame:            4,
			ExportedModified:        5,
			ExportedSameIgnored:     6,
			ExportedModifiedIgnored: 7,
		},
		same:                    0,
		modified:                1,
		sameIgnored:             2,
		modifiedIgnored:         3,
		ExportedSame:            4,
		ExportedModified:        5,
		ExportedSameIgnored:     6,
		ExportedModifiedIgnored: 7,
		recurseInline: s1{
			same:                    0,
			modified:                1,
			sameIgnored:             2,
			modifiedIgnored:         3,
			ExportedSame:            4,
			ExportedModified:        5,
			ExportedSameIgnored:     6,
			ExportedModifiedIgnored: 7,
		},
		recursePtr: &s2{
			same:                    0,
			modified:                1,
			sameIgnored:             2,
			modifiedIgnored:         3,
			ExportedSame:            4,
			ExportedModified:        5,
			ExportedSameIgnored:     6,
			ExportedModifiedIgnored: 7,
		},
	}
	sb := s2{
		s1: s1{
			same:                    0,
			modified:                10,
			sameIgnored:             2,
			modifiedIgnored:         30,
			ExportedSame:            4,
			ExportedModified:        50,
			ExportedSameIgnored:     6,
			ExportedModifiedIgnored: 70,
		},
		same:                    0,
		modified:                10,
		sameIgnored:             2,
		modifiedIgnored:         30,
		ExportedSame:            4,
		ExportedModified:        50,
		ExportedSameIgnored:     6,
		ExportedModifiedIgnored: 70,
		recurseInline: s1{
			same:                    0,
			modified:                10,
			sameIgnored:             2,
			modifiedIgnored:         30,
			ExportedSame:            4,
			ExportedModified:        50,
			ExportedSameIgnored:     6,
			ExportedModifiedIgnored: 70,
		},
		recursePtr: &s2{
			same:                    0,
			modified:                10,
			sameIgnored:             2,
			modifiedIgnored:         30,
			ExportedSame:            4,
			ExportedModified:        50,
			ExportedSameIgnored:     6,
			ExportedModifiedIgnored: 70,
		},
	}

	restoreCompareUnexportedFields := deep.CompareUnexportedFields
	t.Cleanup(func() { deep.CompareUnexportedFields = restoreCompareUnexportedFields })

	deep.CompareUnexportedFields = true

	shouldBeDiffs(t, deep.Equal(sa, sb),
		"s1.modified: 1 != 10",
		"s1.ExportedModified: 5 != 50",
		"modified: 1 != 10",
		"ExportedModified: 5 != 50",
		"recurseInline.modified: 1 != 10",
		"recurseInline.ExportedModified: 5 != 50",
		"recursePtr.modified: 1 != 10",
		"recursePtr.ExportedModified: 5 != 50",
	)
}

func TestNestedStruct(t *testing.T) {
	type s2 struct {
		Nickname string
	}
	type s1 struct {
		Name  string
		Alias s2
	}

	sa := s1{
		Name:  "Robert",
		Alias: s2{Nickname: "Bob"},
	}
	sb := sa

	shouldBeEqual(t, deep.Equal(sa, sb))

	sb.Alias.Nickname = "Bobby"

	shouldBeDiffs(t, deep.Equal(sa, sb), "Alias.Nickname: Bob != Bobby")
}

func TestMap(t *testing.T) {
	ma := map[string]int{
		"foo": 1,
		"bar": 2,
	}

	shouldBeEqual(t, deep.Equal(ma, ma))

	mb := map[string]int{
		"foo": 1,
		"bar": 2,
	}

	shouldBeEqual(t, deep.Equal(ma, mb))

	mb["foo"] = 111
	shouldBeDiffs(t, deep.Equal(ma, mb), "map[foo]: 1 != 111")

	delete(mb, "foo")
	shouldBeDiffs(t, deep.Equal(ma, mb), "map[foo]: 1 != <does not have key>")
	shouldBeDiffs(t, deep.Equal(mb, ma), "map[foo]: <does not have key> != 1")

	var mc map[string]int
	shouldBeDiffs(t, deep.Equal(mb, mc), "map[bar:2] != <nil map>")
	shouldBeDiffs(t, deep.Equal(mc, mb), "<nil map> != map[bar:2]")
}

func TestArray(t *testing.T) {
	a := [3]int{1, 2, 3}

	shouldBeEqual(t, deep.Equal(a, a))

	b := [3]int{1, 2, 3}

	shouldBeEqual(t, deep.Equal(a, b))

	b[2] = 333
	shouldBeDiffs(t, deep.Equal(a, b), "array[2]: 3 != 333")

	c := [3]int{1, 2, 2}
	shouldBeDiffs(t, deep.Equal(a, c), "array[2]: 3 != 2")

	var d [2]int
	shouldBeDiffs(t, deep.Equal(a, d), "[3]int != [2]int")

	e := [12]int{}
	f := [12]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	restoreMaxDiff := deep.MaxDiff
	t.Cleanup(func() { deep.MaxDiff = restoreMaxDiff })

	deep.MaxDiff = 10

	shouldBeDiffs(t, deep.Equal(e, f),
		"array[1]: 0 != 1",
		"array[2]: 0 != 2",
		"array[3]: 0 != 3",
		"array[4]: 0 != 4",
		"array[5]: 0 != 5",
		"array[6]: 0 != 6",
		"array[7]: 0 != 7",
		"array[8]: 0 != 8",
		"array[9]: 0 != 9",
		"array[10]: 0 != 10",
	)
}

func TestSlice(t *testing.T) {
	a := []int{1, 2, 3}

	shouldBeEqual(t, deep.Equal(a, a))

	b := []int{1, 2, 3}

	shouldBeEqual(t, deep.Equal(a, b))

	b[2] = 333
	shouldBeDiffs(t, deep.Equal(a, b), "slice[2]: 3 != 333")

	b = b[0:2]
	shouldBeDiffs(t, deep.Equal(a, b), "slice[2]: 3 != <no value>")
	shouldBeDiffs(t, deep.Equal(b, a), "slice[2]: <no value> != 3")

	var c []int

	shouldBeDiffs(t, deep.Equal(a, c), "[1 2 3] != <nil slice>")
	shouldBeDiffs(t, deep.Equal(c, a), "<nil slice> != [1 2 3]")
}

func TestSiblingSlices(t *testing.T) {
	father := []int{1, 2, 3, 4}
	a := father[0:3]
	b := father[0:3]

	shouldBeEqual(t, deep.Equal(a, b))

	a = father[0:3]
	b = father[0:2]
	shouldBeDiffs(t, deep.Equal(a, b), "slice[2]: 3 != <no value>")
	shouldBeDiffs(t, deep.Equal(b, a), "slice[2]: <no value> != 3")

	a = father[0:2]
	b = father[2:4]
	shouldBeDiffs(t, deep.Equal(a, b),
		"slice[0]: 1 != 3",
		"slice[1]: 2 != 4",
	)

	a = father[0:0]
	b = father[1:1]

	shouldBeEqual(t, deep.Equal(a, b))
	shouldBeEqual(t, deep.Equal(b, a))
}

func TestEmptySlice(t *testing.T) {
	a := []int{1}
	b := []int{}

	// Non-empty is not equal to empty.
	shouldBeDiffs(t, deep.Equal(a, b), "slice[0]: 1 != <no value>")

	// Empty is not equal to non-empty.
	shouldBeDiffs(t, deep.Equal(b, a), "slice[0]: <no value> != 1")

	var c []int

	// Empty is not equal to nil.
	shouldBeDiffs(t, deep.Equal(b, c), "[] != <nil slice>")

	// Nil is not equal to empty.
	shouldBeDiffs(t, deep.Equal(c, b), "<nil slice> != []")
}

func TestNilSlicesAreEmpty(t *testing.T) {
	restoreNilSlicesAreEmpty := deep.NilSlicesAreEmpty
	t.Cleanup(func() { deep.NilSlicesAreEmpty = restoreNilSlicesAreEmpty })

	deep.NilSlicesAreEmpty = true

	a := []int{1}
	b := []int{}

	var c []int

	// Empty is equal to nil.
	shouldBeEqual(t, deep.Equal(b, c))

	// Nil is equal to empty.
	shouldBeEqual(t, deep.Equal(c, b))

	// Non-empty is not equal to nil.
	shouldBeDiffs(t, deep.Equal(a, c), "[1] != <nil slice>")

	// Nil is not equal to non-empty.
	shouldBeDiffs(t, deep.Equal(c, a), "<nil slice> != [1]")

	// Non-empty is not equal to empty.
	shouldBeDiffs(t, deep.Equal(a, b), "slice[0]: 1 != <no value>")

	// Empty is not equal to non-empty.
	shouldBeDiffs(t, deep.Equal(b, a), "slice[0]: <no value> != 1")
}

func TestNilMapsAreEmpty(t *testing.T) {
	restoreNilMapsAreEmpty := deep.NilMapsAreEmpty
	t.Cleanup(func() { deep.NilMapsAreEmpty = restoreNilMapsAreEmpty })

	deep.NilMapsAreEmpty = true

	a := map[int]int{1: 1}
	b := make(map[int]int)
	var c map[int]int

	// Empty is equal to nil.
	shouldBeEqual(t, deep.Equal(b, c))

	// Nil is equal to empty.
	shouldBeEqual(t, deep.Equal(c, b))

	// Non-empty is not equal to nil.
	shouldBeDiffs(t, deep.Equal(a, c), "map[1:1] != <nil map>")

	// Nil is not equal to non-empty.
	shouldBeDiffs(t, deep.Equal(c, a), "<nil map> != map[1:1]")

	// Non-empty is not equal to empty.
	shouldBeDiffs(t, deep.Equal(a, b), "map[1]: 1 != <does not have key>")

	// Empty is not equal to non-empty.
	shouldBeDiffs(t, deep.Equal(b, a), "map[1]: <does not have key> != 1")
}

func TestNilInterface(t *testing.T) {
	type T struct{ i int }

	a := &T{i: 1}
	shouldBeDiffs(t, deep.Equal(nil, a), "<untyped nil> != &{1}")

	shouldBeDiffs(t, deep.Equal(a, nil), "&{1} != <untyped nil>")

	shouldBeEqual(t, deep.Equal(nil, nil))
}

func TestPointer(t *testing.T) {
	type T struct{ i int }

	a, b := &T{i: 1}, &T{i: 1}
	shouldBeEqual(t, deep.Equal(a, b))

	a, b = nil, &T{}
	shouldBeDiffs(t, deep.Equal(a, b), "<nil pointer> != *deep_test.T")

	a, b = &T{}, nil
	shouldBeDiffs(t, deep.Equal(a, b), "*deep_test.T != <nil pointer>")

	a, b = nil, nil
	shouldBeEqual(t, deep.Equal(a, b))
}

func TestTime(t *testing.T) {
	// In an interable kind (i.e. a struct)
	type sTime struct {
		T time.Time
	}

	now := time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC)
	later := now.Add(1 * time.Second)

	s1 := sTime{T: now}
	s2 := sTime{T: later}
	shouldBeDiffs(t, deep.Equal(s1, s2),
		"T: 2009-11-10 23:00:00 +0000 UTC != 2009-11-10 23:00:01 +0000 UTC",
	)

	// Directly
	shouldBeEqual(t, deep.Equal(now, now))

	// https://github.com/go-test/deep/issues/15
	type Time15 struct {
		time.Time
	}
	a15 := Time15{now}
	b15 := Time15{now}
	shouldBeEqual(t, deep.Equal(a15, b15))

	b15 = Time15{later}
	shouldBeDiffs(t, deep.Equal(a15, b15),
		"Time: 2009-11-10 23:00:00 +0000 UTC != 2009-11-10 23:00:01 +0000 UTC",
	)

	// No diff in Equal should not affect diff of other fields (Foo)
	type Time17 struct {
		time.Time
		Foo int
	}
	a17 := Time17{Time: now, Foo: 1}
	b17 := Time17{Time: now, Foo: 2}
	shouldBeDiffs(t, deep.Equal(a17, b17), "Foo: 1 != 2")
}

func TestTimeUnexported(t *testing.T) {
	restoreCompareUnexportedFields := deep.CompareUnexportedFields
	t.Cleanup(func() { deep.CompareUnexportedFields = restoreCompareUnexportedFields })

	deep.CompareUnexportedFields = true

	now := time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC)
	later := now.Add(1 * time.Second)

	// https://github.com/go-test/deep/issues/18
	// Can't call Call() on unexported Value func

	type hiddenTime struct {
		t time.Time
	}
	htA := &hiddenTime{t: now}
	htB := &hiddenTime{t: now}

	shouldBeEqual(t, deep.Equal(htA, htB))

	htB.t = later
	shouldBeDiffs(t, deep.Equal(htA, htB), "t.ext: 63393490800 != 63393490801")

	// This doesn't call time.Time.Equal(), it compares the unexported fields
	// in time.Time, causing a diff like:
	// [t.wall: 13740788835924462040 != 13740788836998203864 t.ext: 1447549 != 1001447549]
	htA.t = time.Now()
	htB.t = htA.t.Add(1 * time.Second)
	diff := deep.Equal(htA, htB)

	expected := reflect.TypeOf(htA.t).NumField() - 1 // loc *Location will always be the same.
	if len(diff) != expected {
		t.Errorf("got %d diffs, expected %d: %s", len(diff), expected, diff)
	}
}

func TestInterface(t *testing.T) {
	defer func() {
		if val := recover(); val != nil {
			t.Fatal("panic:", val)
		}
	}()

	a := map[string]interface{}{
		"foo": map[string]string{
			"bar": "a",
		},
	}
	b := map[string]interface{}{
		"foo": map[string]string{
			"bar": "b",
		},
	}
	shouldBeDiffs(t, deep.Equal(a, b), "map[foo].map[bar]: a != b")

	a["foo"] = 1
	b["foo"] = 1.23

	shouldBeDiffs(t, deep.Equal(a, b), "map[foo]: int != float64")

	type Value struct{ int }
	a["foo"] = &Value{}

	shouldBeDiffs(t, deep.Equal(a, b), "map[foo]: *deep_test.Value != float64")
}

func TestError(t *testing.T) {
	a := errors.New("it broke")
	b := errors.New("it broke")

	shouldBeEqual(t, deep.Equal(a, b))

	b = errors.New("it fell apart")
	shouldBeDiffs(t, deep.Equal(a, b), "it broke != it fell apart")

	// Both errors set
	type tWithError struct {
		Error error
	}

	t1 := tWithError{
		Error: a,
	}
	t2 := tWithError{
		Error: b,
	}
	shouldBeDiffs(t, deep.Equal(t1, t2), "Error: it broke != it fell apart")

	// Both errors nil
	t1 = tWithError{
		Error: nil,
	}
	t2 = tWithError{
		Error: nil,
	}
	shouldBeEqual(t, deep.Equal(t1, t2))

	// One error is nil
	t1 = tWithError{
		Error: errors.New("foo"),
	}
	t2 = tWithError{
		Error: nil,
	}
	shouldBeDiffs(t, deep.Equal(t1, t2), "Error: *errors.errorString != <nil pointer>")
}

func TestErrorWithOtherFields(t *testing.T) {
	a := errors.New("it broke")
	b := errors.New("it broke")

	shouldBeEqual(t, deep.Equal(a, b))

	b = errors.New("it fell apart")
	shouldBeDiffs(t, deep.Equal(a, b), "it broke != it fell apart")

	// Both errors set
	type tWithError struct {
		Error error
		Other string
	}

	t1 := tWithError{
		Error: a,
		Other: "ok",
	}
	t2 := tWithError{
		Error: b,
		Other: "ok",
	}
	shouldBeDiffs(t, deep.Equal(t1, t2), "Error: it broke != it fell apart")

	// Both errors nil
	t1 = tWithError{
		Error: nil,
		Other: "ok",
	}
	t2 = tWithError{
		Error: nil,
		Other: "ok",
	}
	shouldBeEqual(t, deep.Equal(t1, t2))

	// Different Other value
	t1 = tWithError{
		Error: nil,
		Other: "ok",
	}
	t2 = tWithError{
		Error: nil,
		Other: "nope",
	}
	shouldBeDiffs(t, deep.Equal(t1, t2), "Other: ok != nope")

	// Different Other value, same error
	t1 = tWithError{
		Error: a,
		Other: "ok",
	}
	t2 = tWithError{
		Error: a,
		Other: "nope",
	}
	shouldBeDiffs(t, deep.Equal(t1, t2), "Other: ok != nope")
}

type primKindError string

func (e primKindError) Error() string {
	return string(e)
}

func TestErrorPrimitiveKind(t *testing.T) {
	// The primKindError type above is valid and used by Go, e.g.
	// url.EscapeError and url.InvalidHostError. Before fixing this bug
	// (https://github.com/go-test/deep/issues/31), we presumed a and b
	// were ptr or interface (and not nil), so a.Elem() worked. But when
	// a/b are primitive kinds, Elem() causes a panic.
	a := primKindError("abc")
	b := primKindError("abc")
	shouldBeEqual(t, deep.Equal(a, b))
}

func TestNil(t *testing.T) {
	type student struct {
		name string
		age  int
	}

	mark := student{"mark", 10}
	var someNilThing interface{} = nil
	shouldBeDiffs(t, deep.Equal(someNilThing, mark), "<untyped nil> != {mark 10}")

	shouldBeDiffs(t, deep.Equal(mark, someNilThing), "{mark 10} != <untyped nil>")

	shouldBeEqual(t, deep.Equal(someNilThing, someNilThing))
}

type equalReturnsNothing int

func (equalReturnsNothing) Equal(_ equalReturnsWrongType) {}

func TestEqualReturnsNothing(t *testing.T) {
	a := equalReturnsNothing(13)
	b := equalReturnsNothing(42)
	shouldBeDiffs(t, deep.Equal(a, b), "13 != 42")
}

type equalReturnsWrongType int

func (equalReturnsWrongType) Equal(_ equalReturnsWrongType) int {
	return 1
}

func TestEqualReturnsWrongType(t *testing.T) {
	a := equalReturnsWrongType(13)
	b := equalReturnsWrongType(42)
	shouldBeDiffs(t, deep.Equal(a, b), "13 != 42")
}

type boolKind bool

type equalReturnsBoolKind int

func (equalReturnsBoolKind) Equal(_ equalReturnsBoolKind) boolKind {
	return true
}

func TestEqualReturnsBoolKind(t *testing.T) {
	a := equalReturnsBoolKind(13)
	b := equalReturnsBoolKind(42)
	shouldBeEqual(t, deep.Equal(a, b)) // Equal should have overriden the comparison.
}

type ring struct {
	Prev, Next *ring
}

func newRing() *ring {
	r := new(ring)
	r.Prev = r
	r.Next = r
	return r
}

func TestRingList(t *testing.T) {
	a := newRing()
	b := newRing()
	shouldBeEqual(t, deep.Equal(a, b))
}

type oroborous struct {
	Any interface{}
}

func newOroborous() *oroborous {
	o := new(oroborous)
	o.Any = o
	return o
}

func TestOroborous(t *testing.T) {
	a := newOroborous()
	b := newOroborous()
	shouldBeEqual(t, deep.Equal(a, b))
}

func TestUnexportedErrorField(t *testing.T) {
	restoreCompareUnexportedFields := deep.CompareUnexportedFields
	t.Cleanup(func() { deep.CompareUnexportedFields = restoreCompareUnexportedFields })

	deep.CompareUnexportedFields = true

	type S struct {
		err error
	}

	a := &S{err: errors.New("it broke")}
	b := &S{err: errors.New("it broke")}
	shouldBeEqual(t, deep.Equal(a, b))
}

func TestIPAddresses(t *testing.T) {
	a := net.ParseIP("1.2.3.4")
	b := net.ParseIP("1.2.3.4")
	shouldBeEqual(t, deep.Equal(a, b))
}

type hasStringer int

func (hs hasStringer) String() string {
	switch hs {
	case 0:
		return "VALUE_ZERO"
	case 1:
		return "VALUE_ONE"
	}

	return fmt.Sprintf("VALUE(%d)", hs)
}

func TestHasStringer(t *testing.T) {
	a := hasStringer(0)
	b := hasStringer(1)
	shouldBeDiffs(t, deep.Equal(a, b), "VALUE_ZERO != VALUE_ONE")
}

func TestTimePrecision(t *testing.T) {
	restoreTimePrecision := deep.TimePrecision
	t.Cleanup(func() { deep.TimePrecision = restoreTimePrecision })

	deep.TimePrecision = 1 * time.Microsecond

	now := time.Date(2009, 11, 10, 23, 0, 0, 0, time.UTC)
	later := now.Add(123 * time.Nanosecond)

	shouldBeEqual(t, deep.Equal(now, later))

	d1 := 1 * time.Microsecond
	d2 := d1 + 123*time.Nanosecond

	shouldBeEqual(t, deep.Equal(d1, d2))

	restoreCompareUnexportedFields := deep.CompareUnexportedFields
	t.Cleanup(func() { deep.CompareUnexportedFields = restoreCompareUnexportedFields })

	deep.CompareUnexportedFields = true

	type S struct {
		t time.Time
		d time.Duration
	}

	s1 := &S{t: now, d: d1}
	s2 := &S{t: later, d: d2}

	// Since we cannot call `Truncate` on the unexported fields,
	// we will show differences here.
	shouldBeDiffs(t, deep.Equal(s1, s2),
		"t.wall: 0 != 123",
		"d: 1000 != 1123",
	)
}

func TestCompareFuncs(t *testing.T) {
	restoreCompareFunctions := deep.CompareFunctions
	t.Cleanup(func() { deep.CompareFunctions = restoreCompareFunctions })

	deep.CompareFunctions = true

	var f1, f2 func()

	shouldBeEqual(t, deep.Equal(f1, f2))

	f2 = func() {}
	shouldBeDiffs(t, deep.Equal(f1, f2), "<nil func> != <non-nil func>")
	shouldBeDiffs(t, deep.Equal(f2, f1), "<non-nil func> != <nil func>")
	shouldBeDiffs(t, deep.Equal(f2, f2), "<non-nil func> != <non-nil func>")
}
