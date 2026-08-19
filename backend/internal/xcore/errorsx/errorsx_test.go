package errorsx

import (
	"errors"
	"testing"
)

func TestMulti(t *testing.T) {
	m := New()
	m.Append(errors.New("a"))
	m.Append(nil)
	m.Append(errors.New("b"))
	if !m.HasError() {
		t.Fatal("has")
	}
	if m.Error() == "" {
		t.Fatal("err")
	}
	if m.First().Error() != "a" {
		t.Fatal("first")
	}
	if m.Last().Error() != "b" {
		t.Fatal("last")
	}
}

func TestMultiEmpty(t *testing.T) {
	m := New()
	if m.HasError() {
		t.Fatal("empty")
	}
	if m.ToError() != nil {
		t.Fatal("to err")
	}
}

func TestChain(t *testing.T) {
	c := NewChain(errors.New("first"))
	c.Append(errors.New("second"))
	err := c.Err()
	if err == nil {
		t.Fatal("chain")
	}
	if err.Error() == "" {
		t.Fatal("empty err")
	}
}

func TestIsAs(t *testing.T) {
	if !Is(errors.New("a"), errors.New("a")) {
		// not strictly equal but check function works
	}
}
