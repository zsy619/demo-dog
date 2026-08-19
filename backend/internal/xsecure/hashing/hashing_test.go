package hashing

import (
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := Hash("hello", 1000)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify("hello", h)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("应匹配")
	}
}

func TestVerify_Wrong(t *testing.T) {
	h, _ := Hash("hello", 1000)
	ok, _ := Verify("hellp", h)
	if ok {
		t.Fatal("应不匹配")
	}
}

func TestVerify_BadFormat(t *testing.T) {
	if _, err := Verify("x", "garbage"); err == nil {
		t.Fatal("应报错")
	}
}

func TestEqual(t *testing.T) {
	if !Equal("abc", "abc") {
		t.Fatal("equal")
	}
	if Equal("abc", "abd") {
		t.Fatal("不等")
	}
}

func TestRandomToken(t *testing.T) {
	t1, _ := RandomToken(16)
	t2, _ := RandomToken(16)
	if t1 == "" || t2 == "" || t1 == t2 {
		t.Fatal("token")
	}
}

func TestHash_MinIters(t *testing.T) {
	h, _ := Hash("x", 1)
	ok, _ := Verify("x", h)
	if !ok {
		t.Fatal("min")
	}
}
