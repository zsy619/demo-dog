package reflection

import "testing"

type S struct {
	A int
	B string
}

func TestTypeName(t *testing.T) {
	if TypeName(S{1, "x"}) == "" {
		t.Fatal("name")
	}
}

func TestFields(t *testing.T) {
	fs := FieldNames(S{})
	if len(fs) != 2 || fs[0] != "A" {
		t.Fatal("fields", fs)
	}
}

func TestFieldsNonStruct(t *testing.T) {
	if FieldNames(42) != nil {
		t.Fatal("non struct")
	}
}

func TestIsZero(t *testing.T) {
	if !IsZero(0) || !IsZero("") {
		t.Fatal("zero")
	}
	if IsZero(1) {
		t.Fatal("not zero")
	}
}

func TestCopy(t *testing.T) {
	a := S{A: 1, B: "x"}
	var b S
	if err := Copy(&b, &a); err != nil {
		t.Fatal(err)
	}
	if b.A != 1 || b.B != "x" {
		t.Fatal("copy", b)
	}
}

func TestCopyBadPtr(t *testing.T) {
	if err := Copy(S{}, S{}); err == nil {
		t.Fatal("bad")
	}
}
