package deep_test

import (
	"net"
	"strings"
	"testing"

	"github.com/go-test/deep"
)

type equalFoldString string

func (s equalFoldString) Equal(other equalFoldString) bool {
	return strings.EqualFold(string(s), string(other))
}

func TestEqualMethodOnScalar(t *testing.T) {
	if diff := deep.Equal(equalFoldString("FOO"), equalFoldString("foo")); diff != nil {
		t.Errorf("case-insensitive strings should be equal: %v", diff)
	}
	if diff := deep.Equal(equalFoldString("foo"), equalFoldString("bar")); diff == nil {
		t.Error("different strings should not be equal")
	}
}

func TestEqualMethodOnSlice(t *testing.T) {
	a := net.ParseIP("1.2.3.4")
	b := a.To4()
	if diff := deep.Equal(a, b); diff != nil {
		t.Errorf("equivalent IPv4 representations should be equal: %v", diff)
	}
	if diff := deep.Equal(b, a); diff != nil {
		t.Errorf("equivalent IPv4 representations should be equal in reverse: %v", diff)
	}
	if diff := deep.Equal(a, net.ParseIP("1.2.3.5")); diff == nil {
		t.Error("different IP addresses should not be equal")
	}
}
