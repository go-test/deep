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

type mockT struct {
	calledError  bool
	calledFatal  bool
	calledHelper bool
}

func (t *mockT) Error(args ...any) { t.calledError = true }
func (t *mockT) Fatal(args ...any) { t.calledFatal = true }
func (t *mockT) Helper()           { t.calledHelper = true }

func TestDiffer_WithInlineTest(t *testing.T) {
	// No diff, no calls to T except Helper
	m := &mockT{}
	d := deep.NewDiffer(deep.WithInlineTest(deep.InlineTest{T: m}))
	d.Diff("foo", "foo")
	if m.calledHelper == false {
		t.Errorf("did not call T.Helper")
	}
	if m.calledError == true {
		t.Errorf("called T.Error")
	}
	if m.calledFatal == true {
		t.Errorf("called T.Fatal")
	}

	// Diff and by default Error, not Fatal, is called
	m = &mockT{}
	d = deep.NewDiffer(deep.WithInlineTest(deep.InlineTest{T: m}))
	d.Diff("foo", "bar")
	if m.calledHelper == false {
		t.Errorf("did not call T.Helper")
	}
	if m.calledError != true {
		t.Errorf("did not call T.Error")
	}
	if m.calledFatal == true {
		t.Errorf("called T.Fatal")
	}

	// Diff and Fatal
	m = &mockT{}
	d = deep.NewDiffer(deep.WithInlineTest(deep.InlineTest{T: m, Fatal: true}))
	d.Diff("foo", "bar")
	if m.calledHelper == false {
		t.Errorf("did not call T.Helper")
	}
	if m.calledError == true {
		t.Errorf("called T.Error")
	}
	if m.calledFatal != true {
		t.Errorf("did not call T.Fatal")
	}
}
