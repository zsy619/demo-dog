package bitsetx

import "testing"

func TestNegativeIndex(t *testing.T) {
	b := New(64)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Set/Get with negative index should not panic: %v", r)
		}
	}()
	b.Set(-1)
	b.Get(-1)
	b.Clear(-1)
}
