package foreachx

import (
	"sync/atomic"
	"testing"
)

func TestForEach(t *testing.T) {
	var n atomic.Int32
	items := make([]int, 100)
	ForEach(items, 8, func(_ int, _ int) { n.Add(1) })
	if n.Load() != 100 {
		t.Fatal("fe", n.Load())
	}
}

func TestForEachEmpty(t *testing.T) {
	ForEach([]int{}, 4, func(_ int, _ int) {
		t.Fatal("应不调用")
	})
}

func TestFilter(t *testing.T) {
	r := Filter([]int{1, 2, 3, 4}, func(v int) bool { return v%2 == 0 })
	if len(r) != 2 || r[0] != 2 {
		t.Fatal("filter", r)
	}
}

func TestReduce(t *testing.T) {
	s := Reduce([]int{1, 2, 3}, 0, func(a, v int) int { return a + v })
	if s != 6 {
		t.Fatal("reduce", s)
	}
}
