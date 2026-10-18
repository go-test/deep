package deep_test

import (
	"math"
	"testing"

	"github.com/go-test/deep"
)

type equalReturnsInt struct{ Value int }

func (equalReturnsInt) Equal(equalReturnsInt) int {
	panic("Equal with a non-bool result must not be called")
}

type equalReturnsNothing struct{ Value int }

func (equalReturnsNothing) Equal(equalReturnsNothing) {
	panic("Equal without a result must not be called")
}

type equalReturnsTwo struct{ Value int }

func (equalReturnsTwo) Equal(equalReturnsTwo) (bool, error) {
	panic("Equal with multiple results must not be called")
}

type equalReturnsNamedBool string
type equalityBool bool

func (s equalReturnsNamedBool) Equal(other equalReturnsNamedBool) equalityBool {
	return equalityBool(len(s) == len(other))
}

func TestEqualMethodReturnSignature(t *testing.T) {
	for _, test := range []struct {
		name string
		a, b interface{}
	}{
		{"int result", equalReturnsInt{1}, equalReturnsInt{2}},
		{"no result", equalReturnsNothing{1}, equalReturnsNothing{2}},
		{"two results", equalReturnsTwo{1}, equalReturnsTwo{2}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if diff := deep.Equal(test.a, test.a); diff != nil {
				t.Error("identical values should be equal:", diff)
			}
			if diff := deep.Equal(test.a, test.b); diff == nil {
				t.Error("different fields should produce a diff")
			}
		})
	}
}

func TestEqualMethodNamedBoolResult(t *testing.T) {
	if diff := deep.Equal(equalReturnsNamedBool("ab"), equalReturnsNamedBool("cd")); diff != nil {
		t.Error("existing Equal methods returning a named bool should be honored:", diff)
	}
	if diff := deep.Equal(equalReturnsNamedBool("ab"), equalReturnsNamedBool("c")); diff == nil {
		t.Error("Equal returning false should produce a diff")
	}
}

func TestSignedZero(t *testing.T) {
	negativeZero := math.Copysign(0, -1)
	for _, test := range []struct {
		name string
		a, b interface{}
	}{
		{"float64", float64(0), negativeZero},
		{"float64 reversed", negativeZero, float64(0)},
		{"float32", float32(0), float32(negativeZero)},
		{"float32 reversed", float32(negativeZero), float32(0)},
		{"nested", []float64{0}, []float64{negativeZero}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if diff := deep.Equal(test.a, test.b); diff != nil {
				t.Error("positive and negative zero should be equal:", diff)
			}
		})
	}
}
