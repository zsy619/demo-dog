package smoke

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRun_Pass(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf)
	h.Add("a", func() error { return nil })
	h.Add("b", func() error { return nil })
	results := h.Run()
	if len(results) != 2 {
		t.Fatal("len")
	}
	if results[0].Name != "a" {
		t.Fatal("name")
	}
}

func TestRun_Fail(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf)
	h.Add("a", func() error { return errors.New("boom") })
	results := h.Run()
	if results[0].OK {
		t.Fatal("should fail")
	}
	if results[0].Err == nil || results[0].Err.Error() != "boom" {
		t.Fatal("err")
	}
}

func TestSummary(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf)
	h.Add("a", func() error { return nil })
	h.Add("b", func() error { return errors.New("x") })
	h.Run()
	s := h.Summary()
	if s.Total != 2 || s.Passed != 1 || s.Failed != 1 {
		t.Fatal("summary")
	}
}

func TestReport(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf)
	h.Add("a", func() error { return nil })
	h.Run()
	h.Report()
	if !strings.Contains(buf.String(), "passed") {
		t.Fatal("report")
	}
}

func TestFailFast(t *testing.T) {
	var buf bytes.Buffer
	h := New(&buf).WithFailFast()
	h.Add("a", func() error { return errors.New("fail") })
	h.Add("b", func() error { return nil })
	results := h.Run()
	if len(results) != 1 {
		t.Fatal("failfast")
	}
}

type fakeT struct {
	errs []string
}

func (f *fakeT) Errorf(format string, args ...any) {
	f.errs = append(f.errs, format)
}

func TestAssert_True(t *testing.T) {
	ft := &fakeT{}
	a := NewAssert(ft)
	a.True(true, "x")
	if len(ft.errs) != 0 {
		t.Fatal("unexpected")
	}
	a.True(false, "y")
	if len(ft.errs) != 1 {
		t.Fatal("expected")
	}
}

func TestAssert_Nil(t *testing.T) {
	ft := &fakeT{}
	a := NewAssert(ft)
	a.Nil(nil, "x")
	a.Nil(errors.New("x"), "y")
	if len(ft.errs) != 1 {
		t.Fatal("nil")
	}
}

func TestAssert_Equal(t *testing.T) {
	ft := &fakeT{}
	a := NewAssert(ft)
	a.Equal(1, 1, "x")
	a.Equal(1, 2, "y")
	if len(ft.errs) != 1 {
		t.Fatal("eq")
	}
}

func TestNoErr(t *testing.T) {
	if NoErr(func() error { return nil }) != nil {
		t.Fatal("noerr")
	}
}

func TestErrIs(t *testing.T) {
	sentinel := errors.New("sentinel")
	wrapped := fmt.Errorf("wrap: %w", sentinel)
	if !ErrIs(wrapped, sentinel) {
		t.Fatal("is")
	}
	if ErrIs(nil, sentinel) {
		t.Fatal("nil is not")
	}
}
