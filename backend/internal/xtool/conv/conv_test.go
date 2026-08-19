package conv

import (
	"errors"
	"testing"
)

func TestToString(t *testing.T) {
	if ToString("a") != "a" {
		t.Fatal("string")
	}
	if ToString(123) != "123" {
		t.Fatal("int")
	}
	if ToString(true) != "true" {
		t.Fatal("bool")
	}
	if ToString(nil) != "" {
		t.Fatal("nil")
	}
	if ToString(errors.New("x")) != "x" {
		t.Fatal("error")
	}
}

func TestToInt(t *testing.T) {
	if n, _ := ToInt("42"); n != 42 {
		t.Fatal("str")
	}
	if n, _ := ToInt(int64(99)); n != 99 {
		t.Fatal("i64")
	}
	if n, _ := ToInt(true); n != 1 {
		t.Fatal("bool")
	}
	if _, err := ToInt(struct{}{}); err == nil {
		t.Fatal("应报错")
	}
}

func TestMustToInt(t *testing.T) {
	if MustToInt("5") != 5 {
		t.Fatal("must")
	}
	if MustToInt("x") != 0 {
		t.Fatal("fallback")
	}
}

func TestToBool(t *testing.T) {
	if !ToBool("true") {
		t.Fatal("true")
	}
	if ToBool("false") {
		t.Fatal("false")
	}
	if !ToBool(1) {
		t.Fatal("1")
	}
	if ToBool(0) {
		t.Fatal("0")
	}
}

func TestToFloat64(t *testing.T) {
	if v, _ := ToFloat64("3.14"); v < 3.13 || v > 3.15 {
		t.Fatal("float")
	}
}

type sample struct {
	A int
	B string
}

func TestToMap(t *testing.T) {
	m := ToMap(sample{A: 1, B: "x"})
	if m["A"].(int) != 1 || m["B"].(string) != "x" {
		t.Fatal("map")
	}
}

func TestToMap_NotStruct(t *testing.T) {
	if ToMap(1) != nil {
		t.Fatal("应 nil")
	}
}

func TestOrDefault(t *testing.T) {
	if OrDefault("", "x") != "x" {
		t.Fatal("def")
	}
	if OrDefault("a", "x") != "a" {
		t.Fatal("keep")
	}
}
