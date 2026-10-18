package deep_test

import (
	"testing"

	"github.com/go-test/deep"
)

func TestUintptr(t *testing.T) {
	if diff := deep.Equal(uintptr(1), uintptr(2)); len(diff) != 1 || diff[0] != "1 != 2" {
		t.Fatalf("different uintptr values: %v", diff)
	}
	if diff := deep.Equal(uintptr(2), uintptr(2)); diff != nil {
		t.Fatalf("equal uintptr values: %v", diff)
	}
}

func TestUintptrStructField(t *testing.T) {
	type value struct{ Handle uintptr }
	if diff := deep.Equal(value{1}, value{2}); len(diff) != 1 || diff[0] != "Handle: 1 != 2" {
		t.Fatalf("different uintptr fields: %v", diff)
	}
}
