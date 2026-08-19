package convx

import "testing"

func TestAtoi(t *testing.T) {
	v, err := Atoi("42")
	if err != nil || v != 42 {
		t.Fatal("atoi", v)
	}
}

func TestAtoiDefault(t *testing.T) {
	if AtoiDefault("bad", 100) != 100 {
		t.Fatal("def")
	}
}

func TestAtoi64(t *testing.T) {
	v, _ := Atoi64("123")
	if v != 123 {
		t.Fatal("64")
	}
}

func TestFloat(t *testing.T) {
	v, _ := ParseFloat("3.14")
	if v != 3.14 {
		t.Fatal("float")
	}
}

func TestBool(t *testing.T) {
	v, _ := ParseBool("true")
	if !v {
		t.Fatal("bool")
	}
}

func TestEmpty(t *testing.T) {
	if _, err := Atoi(""); err == nil {
		t.Fatal("empty")
	}
}
