package segmentx

import (
	"math/rand"
	"testing"
)

func TestAddStress(t *testing.T) {
	s := New()
	ops := 500
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < ops; i++ {
		lo := int64(rng.Intn(100))
		hi := lo + int64(rng.Intn(20)+1)
		s.Add(lo, hi)
	}
	// 不变量：segments 必须有序且不重叠
	all := s.All()
	for i := 1; i < len(all); i++ {
		if all[i].Lo < all[i-1].Lo {
			t.Fatalf("无序：%v", all)
		}
		if all[i].Lo < all[i-1].Hi {
			t.Fatalf("重叠：%v 和 %v", all[i-1], all[i])
		}
	}
}

func TestContainsStress(t *testing.T) {
	s := New()
	s.Add(0, 10)
	s.Add(20, 30)
	s.Add(15, 25)
	cases := []struct{ v int64; want bool }{
		{-1, false}, {0, true}, {9, true}, {10, false}, {14, false},
		{15, true}, {19, true}, {20, true}, {24, true}, {25, true}, {29, true}, {30, false}, {100, false},
	}
	for _, c := range cases {
		if got := s.Contains(c.v); got != c.want {
			t.Fatalf("Contains(%d)=%v want %v", c.v, got, c.want)
		}
	}
}

func TestAllSorted(t *testing.T) {
	s := New()
	for _, p := range [][2]int64{{5, 10}, {0, 3}, {20, 30}, {10, 15}} {
		s.Add(p[0], p[1])
	}
	all := s.All()
	for i := 1; i < len(all); i++ {
		if all[i].Lo < all[i-1].Lo {
			t.Fatalf("All 未排序")
		}
	}
	// sort ok
}
