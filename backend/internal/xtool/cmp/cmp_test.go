package cmp

import "testing"

func TestCompare(t *testing.T) {
	if Compare(1, 2, LessInt) != -1 {
		t.Fatal("lt")
	}
	if Compare(2, 1, LessInt) != 1 {
		t.Fatal("gt")
	}
	if Compare(1, 1, LessInt) != 0 {
		t.Fatal("eq")
	}
}

func TestLessInt(t *testing.T) {
	if !LessInt(1, 2) || LessInt(2, 1) {
		t.Fatal("int")
	}
}

func TestLessString(t *testing.T) {
	if !LessString("a", "b") || LessString("b", "a") {
		t.Fatal("str")
	}
}

func TestMinMax(t *testing.T) {
	if Min(1, 2, LessInt) != 1 {
		t.Fatal("min")
	}
	if Max(1, 2, LessInt) != 2 {
		t.Fatal("max")
	}
}

func TestEqual(t *testing.T) {
	if !Equal(1, 1) {
		t.Fatal("eq")
	}
	if Equal(1, 2) {
		t.Fatal("neq")
	}
}
