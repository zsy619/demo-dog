package uuid

import (
	"testing"
)

func TestNew(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 36 {
		t.Fatal("len")
	}
}

func TestBytes_Version(t *testing.T) {
	b, _ := Bytes()
	if Version(b) != 4 {
		t.Fatal("version")
	}
}

func TestParseRoundtrip(t *testing.T) {
	s, _ := New()
	b, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if Format(b) != s {
		t.Fatal("roundtrip")
	}
}

func TestParse_Short(t *testing.T) {
	if _, err := Parse("short"); err == nil {
		t.Fatal("应报错")
	}
}

func TestIsValid(t *testing.T) {
	s, _ := New()
	if !IsValid(s) {
		t.Fatal("valid")
	}
	if IsValid("not-a-uuid") {
		t.Fatal("invalid")
	}
}

func TestMustNew(t *testing.T) {
	s := MustNew()
	if len(s) != 36 {
		t.Fatal("must")
	}
}

func TestDistinctness(t *testing.T) {
	a, _ := New()
	b, _ := New()
	if a == b {
		t.Fatal("应不同")
	}
}
