package errorsx

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(KindNotFound, "missing")
	if !IsKind(err, KindNotFound) {
		t.Fatal("kind")
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("inner")
	err := Wrap(KindInternal, inner, "wrap")
	if errors.Unwrap(err) != inner {
		t.Fatal("unwrap")
	}
}

func TestKindString(t *testing.T) {
	if KindTimeout.String() != "timeout" {
		t.Fatal("str")
	}
}

func TestCode(t *testing.T) {
	err := &E{Kind: KindInternal, Code: "E1"}
	if Code(err) != "E1" {
		t.Fatal("code")
	}
	if Code(errors.New("x")) != "" {
		t.Fatal("empty")
	}
}

func TestKindOf(t *testing.T) {
	err := errors.New("x")
	if KindOf(err) != KindUnknown {
		t.Fatal("unknown")
	}
}

func TestMulti(t *testing.T) {
	m := &Multi{}
	m.Append(errors.New("a"))
	m.Append(nil)
	m.Append(errors.New("b"))
	if !m.HasErrors() {
		t.Fatal("has")
	}
	if m.Err() == nil {
		t.Fatal("err")
	}
}

func TestMultiEmpty(t *testing.T) {
	m := &Multi{}
	if m.Err() != nil {
		t.Fatal("empty")
	}
}
