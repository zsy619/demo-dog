package idset

import "testing"

func TestMinZeroBug(t *testing.T) {
	s := New()
	s.Add(0)
	min, ok := s.Min()
	if !ok {
		t.Fatalf("0 存在时 Min 应返回 ok=true")
	}
	if min != 0 {
		t.Fatalf("min 应为 0，得到 %d", min)
	}
}
