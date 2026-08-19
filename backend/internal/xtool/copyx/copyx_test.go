package copyx

import "testing"

type point struct {
	X int
	Y int
}

func TestDeep(t *testing.T) {
	a := point{X: 1, Y: 2}
	b, err := Deep(a)
	if err != nil {
		t.Fatal(err)
	}
	if b.X != 1 || b.Y != 2 {
		t.Fatal("v")
	}
}

func TestDeep_Independent(t *testing.T) {
	a := point{X: 1, Y: 2}
	b := MustDeep(a)
	b.X = 99
	if a.X != 1 {
		t.Fatal("ind")
	}
}

func TestDeepSlice(t *testing.T) {
	a := []int{1, 2, 3}
	b := MustDeep(a)
	b[0] = 99
	if a[0] != 1 {
		t.Fatal("slice")
	}
}
