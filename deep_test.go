package deep

import (
	"errors"
	"testing"
	"time"
)

func TestString(t *testing.T) {
	diff := Equal("foo", "foo")
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal("foo", "bar")
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestFloat(t *testing.T) {
	diff := Equal(1.1, 1.1)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(1.1234561, 1.1234562)
	if diff == nil {
		t.Error("no diff")
	}

	defaultFloatPrecision := FloatPrecision
	FloatPrecision = 6
	defer func() { FloatPrecision = defaultFloatPrecision }()

	diff = Equal(1.1234561, 1.1234562)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(1.123456, 1.123457)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "1.123456 != 1.123457" {
		t.Error("wrong diff:", diff[0])
	}

}

func TestInt(t *testing.T) {
	diff := Equal(1, 1)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(1, 2)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "1 != 2" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestUint(t *testing.T) {
	diff := Equal(uint(2), uint(2))
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(uint(2), uint(3))
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "2 != 3" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestBool(t *testing.T) {
	diff := Equal(true, true)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(false, false)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(true, false)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "true != false" { // unless you're fipar
		t.Error("wrong diff:", diff[0])
	}
}

func TestTypeMismatch(t *testing.T) {
	type T1 int // same type kind (int)
	type T2 int // but different type
	var t1 T1 = 1
	var t2 T2 = 1
	diff := Equal(t1, t2)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "deep.T1 != deep.T2" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestKindMismatch(t *testing.T) {
	LogErrors = true

	var x int = 100
	var y float64 = 100
	diff := Equal(x, y)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "int != float64" {
		t.Error("wrong diff:", diff[0])
	}

	LogErrors = false
}

func TestDeepRecursion(t *testing.T) {
	MaxDepth = 2
	defer func() { MaxDepth = 10 }()

	type s3 struct {
		S int
	}
	type s2 struct {
		S s3
	}
	type s1 struct {
		S s2
	}
	foo := map[string]s1{
		"foo": { // 1
			S: s2{ // 2
				S: s3{ // 3
					S: 42, // 4
				},
			},
		},
	}
	bar := map[string]s1{
		"foo": {
			S: s2{
				S: s3{
					S: 100,
				},
			},
		},
	}
	diff := Equal(foo, bar)

	defaultMaxDepth := MaxDepth
	MaxDepth = 4
	defer func() { MaxDepth = defaultMaxDepth }()

	diff = Equal(foo, bar)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo].S.S.S: 42 != 100" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestMaxDiff(t *testing.T) {
	a := []int{1, 2, 3, 4, 5, 6, 7}
	b := []int{0, 0, 0, 0, 0, 0, 0}

	defaultMaxDiff := MaxDiff
	MaxDiff = 3
	defer func() { MaxDiff = defaultMaxDiff }()

	diff := Equal(a, b)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != MaxDiff {
		t.Errorf("got %d diffs, execpted %d", len(diff), MaxDiff)
	}

	defaultCompareUnexportedFields := CompareUnexportedFields
	CompareUnexportedFields = true
	defer func() { CompareUnexportedFields = defaultCompareUnexportedFields }()
	type fiveFields struct {
		a int // unexported fields require ^
		b int
		c int
		d int
		e int
	}
	t1 := fiveFields{1, 2, 3, 4, 5}
	t2 := fiveFields{0, 0, 0, 0, 0}
	diff = Equal(t1, t2)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != MaxDiff {
		t.Errorf("got %d diffs, execpted %d", len(diff), MaxDiff)
	}

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
	diff = Equal(m1, m2)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != MaxDiff {
		t.Log(diff)
		t.Errorf("got %d diffs, execpted %d", len(diff), MaxDiff)
	}

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
	diff = Equal(m1, m2)
	if diff == nil {
		t.Fatal("no diffs")
	}
	if len(diff) != MaxDiff {
		t.Log(diff)
		t.Errorf("got %d diffs, execpted %d", len(diff), MaxDiff)
	}
}

func TestNotHandled(t *testing.T) {
	a := func(int) {}
	b := func(int) {}
	diff := Equal(a, b)
	if len(diff) > 0 {
		t.Error("got diffs:", diff)
	}
}

func TestStruct(t *testing.T) {
	type s1 struct {
		id     int
		Name   string
		Number int
	}
	sa := s1{
		id:     1,
		Name:   "foo",
		Number: 2,
	}
	sb := sa
	diff := Equal(sa, sb)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	sb.Name = "bar"
	diff = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}

	sb.Number = 22
	diff = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 2 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
	if diff[1] != "Number: 2 != 22" {
		t.Error("wrong diff:", diff[1])
	}

	sb.id = 11
	diff = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 2 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Name: foo != bar" {
		t.Error("wrong diff:", diff[0])
	}
	if diff[1] != "Number: 2 != 22" {
		t.Error("wrong diff:", diff[1])
	}
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
	diff := Equal(sa, sb)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	sb.Alias.Nickname = "Bobby"
	diff = Equal(sa, sb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "Alias.Nickname: Bob != Bobby" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestMap(t *testing.T) {
	ma := map[string]int{
		"foo": 1,
		"bar": 2,
	}
	mb := map[string]int{
		"foo": 1,
		"bar": 2,
	}
	diff := Equal(ma, mb)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(ma, ma)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	mb["foo"] = 111
	diff = Equal(ma, mb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo]: 1 != 111" {
		t.Error("wrong diff:", diff[0])
	}

	delete(mb, "foo")
	diff = Equal(ma, mb)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo]: 1 != <does not have key>" {
		t.Error("wrong diff:", diff[0])
	}

	diff = Equal(mb, ma)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "map[foo]: <does not have key> != 1" {
		t.Error("wrong diff:", diff[0])
	}

	var mc map[string]int
	diff = Equal(ma, mc)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	// handle hash order randomness
	if diff[0] != "map[foo:1 bar:2] != <nil map>" && diff[0] != "map[bar:2 foo:1] != <nil map>" {
		t.Error("wrong diff:", diff[0])
	}

	diff = Equal(mc, ma)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "<nil map> != map[foo:1 bar:2]" && diff[0] != "<nil map> != map[bar:2 foo:1]" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestSlice(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{1, 2, 3}

	diff := Equal(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	diff = Equal(a, a)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	b[2] = 333
	diff = Equal(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "slice[2]: 3 != 333" {
		t.Error("wrong diff:", diff[0])
	}

	b = b[0:2]
	diff = Equal(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "slice[2]: 3 != <no value>" {
		t.Error("wrong diff:", diff[0])
	}

	diff = Equal(b, a)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "slice[2]: <no value> != 3" {
		t.Error("wrong diff:", diff[0])
	}

	var c []int
	diff = Equal(a, c)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "[1 2 3] != <nil slice>" {
		t.Error("wrong diff:", diff[0])
	}

	diff = Equal(c, a)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "<nil slice> != [1 2 3]" {
		t.Error("wrong diff:", diff[0])
	}
}

func TestPointer(t *testing.T) {
	type T struct {
		i int
	}
	a := &T{i: 1}
	b := &T{i: 1}
	diff := Equal(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}

	a = nil
	diff = Equal(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "<nil pointer> != deep.T" {
		t.Error("wrong diff:", diff[0])
	}

	a = b
	b = nil
	diff = Equal(a, b)
	if diff == nil {
		t.Fatal("no diff")
	}
	if len(diff) != 1 {
		t.Error("too many diff:", diff)
	}
	if diff[0] != "deep.T != <nil pointer>" {
		t.Error("wrong diff:", diff[0])
	}

	a = nil
	b = nil
	diff = Equal(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}
}

func TestTime(t *testing.T) {
	// In an interable kind (i.e. a struct)
	type sTime struct {
		T time.Time
	}
	now := time.Now()
	got := sTime{T: now}
	expect := sTime{T: now.Add(1 * time.Second)}
	diff := Equal(got, expect)
	if len(diff) != 1 {
		t.Error("expected 1 diff:", diff)
	}

	// Directly
	a := now
	b := now
	diff = Equal(a, b)
	if len(diff) > 0 {
		t.Error("should be equal:", diff)
	}
}

func TestInterface(t *testing.T) {
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
	diff := Equal(a, b)
	if len(diff) == 0 {
		t.Fatalf("expected 1 diff, got zero")
	}
	if len(diff) != 1 {
		t.Errorf("expected 1 diff, got %d", len(diff))
	}
}

func TestError(t *testing.T) {
	a := errors.New("it broke")
	b := errors.New("it broke")

	diff := Equal(a, b)
	if len(diff) != 0 {
		t.Fatalf("expected zero diffs, got %d: %s", len(diff), diff)
	}

	b = errors.New("it fell apart")
	diff = Equal(a, b)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff))
	}
	if diff[0] != "it broke != it fell apart" {
		t.Errorf("got '%s', expected 'it broke != it fell apart'", diff[0])
	}

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
	diff = Equal(t1, t2)
	if len(diff) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diff))
	}
	if diff[0] != "Error: it broke != it fell apart" {
		t.Errorf("got '%s', expected 'Error: it broke != it fell apart'", diff[0])
	}

	// Both errors nil
	t1 = tWithError{
		Error: nil,
	}
	t2 = tWithError{
		Error: nil,
	}
	diff = Equal(t1, t2)
	if len(diff) != 0 {
		t.Log(diff)
		t.Fatalf("expected 0 diff, got %d", len(diff))
	}

	// One error is nil
	t1 = tWithError{
		Error: errors.New("foo"),
	}
	t2 = tWithError{
		Error: nil,
	}
	diff = Equal(t1, t2)
	if len(diff) != 1 {
		t.Log(diff)
		t.Fatalf("expected 1 diff, got %d", len(diff))
	}
	if diff[0] != "Error: *errors.errorString != <nil pointer>" {
		t.Errorf("got '%s', expected 'Error: *errors.errorString != <nil pointer>'", diff[0])
	}
}

func TestNil(t *testing.T) {
	type student struct {
		name string
		age  int
	}

	mark := student{"mark", 10}
	var someNilThing interface{} = nil
	diff := Equal(someNilThing, mark)
	if diff == nil {
		t.Error("Nil value to comparision should not be equal")
	}
	diff = Equal(mark, someNilThing)
	if diff == nil {
		t.Error("Nil value to comparision should not be equal")
	}
	diff = Equal(someNilThing, someNilThing)
	if diff != nil {
		t.Error("Nil value to comparision should not be equal")
	}
}
