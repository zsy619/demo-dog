package entropy

import (
	"math"
	"testing"
)

func TestShannon(t *testing.T) {
	e := Shannon([]byte("aaaa"))
	if e != 0 {
		t.Fatal("all same", e)
	}
}

func TestShannonBinary(t *testing.T) {
	e := Shannon([]byte("ab"))
	if math.Abs(e-1) > 1e-9 {
		t.Fatal("ab", e)
	}
}

func TestEmpty(t *testing.T) {
	if Shannon(nil) != 0 {
		t.Fatal("empty")
	}
}

func TestString(t *testing.T) {
	if ShannonString("abc") <= 0 {
		t.Fatal("str")
	}
}

func TestStrength(t *testing.T) {
	if Strength(20) != "very weak" {
		t.Fatal("weak")
	}
	if Strength(40) != "medium" {
		t.Fatal("med")
	}
	if Strength(80) != "strong" {
		t.Fatal("strong")
	}
	if Strength(200) != "very strong" {
		t.Fatal("vs")
	}
}
