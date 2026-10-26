package deep_test

import (
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

func TestDiffer(t *testing.T) {
	d := deep.NewDiffer()

	diff1 := d.Equal("foo", "bar")
	if diff1 == nil {
		t.Error("no diff")
	}

	// diff1 and diff3 are NOT equal
	diff3 := d.Equal("a", "b")
	if diff3 == nil {
		t.Error("should not be equal")
	}
}

type mockT struct {
	calledError  bool
	calledFatal  bool
	calledHelper bool
	errFormat    string
	errArgs      []any
}

func (t *mockT) Errorf(format string, args ...any) {
	t.calledError = true
	t.errFormat = format
	t.errArgs = args
}
func (t *mockT) Fatalf(format string, args ...any) { t.calledFatal = true }
func (t *mockT) Helper()                           { t.calledHelper = true }

func TestDiffer_WithInlineTesting(t *testing.T) {
	// No diff, no calls to T except Helper
	m := &mockT{}
	d := deep.NewDiffer(deep.WithInlineTesting(deep.InlineTesting{T: m}))
	d.Equal("foo", "foo")
	if m.calledHelper == false {
		t.Errorf("did not call T.Helper")
	}
	if m.calledError == true {
		t.Errorf("called T.Error")
	}
	if m.calledFatal == true {
		t.Errorf("called T.Fatal")
	}

	// Equal and by default Error, not Fatal, is called
	m = &mockT{}
	d = deep.NewDiffer(deep.WithInlineTesting(deep.InlineTesting{T: m}))
	d.Equal("foo", "bar")
	if m.calledHelper == false {
		t.Errorf("did not call T.Helper")
	}
	if m.calledError != true {
		t.Errorf("did not call T.Error")
	}
	if m.calledFatal == true {
		t.Errorf("called T.Fatal")
	}

	// Equal and Fatal
	m = &mockT{}
	d = deep.NewDiffer(deep.WithInlineTesting(deep.InlineTesting{T: m, Fatal: true}))
	d.Equal("foo", "bar")
	if m.calledHelper == false {
		t.Errorf("did not call T.Helper")
	}
	if m.calledError == true {
		t.Errorf("called T.Error")
	}
	if m.calledFatal != true {
		t.Errorf("did not call T.Fatal")
	}

	// Convenience wrapper
	m = &mockT{}
	d = deep.Testing(m)
	d.Equal("foo", "bar")
	if m.calledHelper == false {
		t.Errorf("did not call T.Helper")
	}

	// Test DiffFormatter and verify that literal "%" in the diff string stay
	// literal by being replaced with %% in Errorf/Fatalf format
	m = &mockT{}
	d = deep.NewDiffer(deep.WithInlineTesting(deep.InlineTesting{
		T: m,
		DiffFormatter: func(diff deep.Diff) string {
			return fmt.Sprintf("<%s>", diff) // <--
		},
	}))
	d.Equalf("%d", "1%s", "foo %d: {diff}", 9)
	errfmt := `foo %d: <[%%d != 1%%s]>` // %% = literal % in Errorf/Fatalf
	if m.errFormat != errfmt {
		t.Errorf("got error format '%s', expected '%s'", m.errFormat, errfmt)
	}
	if len(m.errArgs) != 1 || m.errArgs[0].(int) != 9 {
		t.Errorf("got erorr args %v, expected [9]", m.errArgs)
	}

	// Equalf should work without DiffPlaceholder {diff}
	m = &mockT{}
	d = deep.NewDiffer(deep.WithInlineTesting(deep.InlineTesting{
		T: m,
	}))
	errfmt = `goodbye, diff`
	d.Equalf("a", "b", errfmt)
	if m.errFormat != errfmt {
		t.Errorf("got error format '%s', expected '%s'", m.errFormat, errfmt)
	}
	if len(m.errArgs) != 0 {
		t.Errorf("got error args, expected none: %v", m.errArgs)
	}
}
