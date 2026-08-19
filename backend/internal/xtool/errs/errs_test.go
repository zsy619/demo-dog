package errs

import "testing"

func TestNew(t *testing.T) {
	e := New(42, "bad")
	if e.Code != 42 || e.Message != "bad" {
		t.Fatal("new")
	}
}

func TestWrap(t *testing.T) {
	e := Wrap(1, nil, "wrap")
	if e.Cause != nil {
		t.Fatal("cause")
	}
}

func TestError(t *testing.T) {
	e := New(7, "msg")
	if e.Error() == "" {
		t.Fatal("err")
	}
}

func TestCodeOf(t *testing.T) {
	if CodeOf(New(99, "x")) != 99 {
		t.Fatal("code")
	}
}

func TestCodeOf_Other(t *testing.T) {
	if CodeOf(nil) != 0 {
		t.Fatal("nil")
	}
}
