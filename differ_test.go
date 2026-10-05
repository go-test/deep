package deep_test

import (
	"testing"

	"github.com/go-test/deep"
)

func TestDiffer(t *testing.T) {
	d := deep.NewDiffer()

	diff1 := d.Diff("foo", "bar")
	if diff1 == nil {
		t.Error("no diff")
	}

	s := diff1.ToSlice()
	if len(s) != 1 {
		t.Errorf("got slice len %d, expected 1", len(s))
	}

	diff2 := d.Diff("foo", "bar")
	if diff2 == nil {
		t.Error("no diff")
	}

	// diff1 and diff2 are equal (i.e. same diff: foo != bar)
	eq := diff1.Equal(diff2)
	if eq != true {
		t.Error("diffs not equal")
	}

	// diff1 and diff3 are NOT equal
	diff3 := d.Diff("a", "b")
	if diff3 == nil {
		t.Error("should not be equal")
	}
	eq = diff1.Equal(diff3)
	if eq == true {
		t.Error("diffs should not be equal")
	}

	// diff4 isn't a diff, so it's not equal to diff1
	diff4 := d.Diff("ok", "ok")
	if diff4 != nil {
		t.Error("should be equal")
	}
	eq = diff1.Equal(diff4)
	if eq == true {
		t.Error("should not be equal")
	}
}
