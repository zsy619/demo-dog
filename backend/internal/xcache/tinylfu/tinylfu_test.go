package tinylfu

import (
	"testing"
)

func TestSketch_Increment(t *testing.T) {
	s := NewSketch(64)
	for i := 0; i < 10; i++ {
		s.Increment([]byte("k"))
	}
	if v := s.Estimate([]byte("k")); v < 5 {
		t.Fatal("频率过低:", v)
	}
}

func TestSketch_EstimateUnknown(t *testing.T) {
	s := NewSketch(64)
	if v := s.Estimate([]byte("missing")); v > 0 {
		t.Fatal("missing 不应有计数")
	}
}

func TestCache_PutGet(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	if v, ok := c.Get("a"); !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestCache_Evict(t *testing.T) {
	c := New(2)
	// 先各 Put 一次让 sketch 计数达到较高值
	c.Put("a", 1)
	c.Put("b", 2)
	for i := 0; i < 20; i++ {
		c.Put("a", 1)
		c.Put("b", 2)
	}
	c.Put("c", 3)
	if c.Len() > 2 {
		t.Fatal("应不超出容量")
	}
}

func TestCache_GetMovesToFront(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")
	c.Put("c", 3)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a 应存活")
	}
}

func TestCache_Clear(t *testing.T) {
	c := New(2)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestSketch_Reset(t *testing.T) {
	s := NewSketch(64)
	for i := 0; i < 20; i++ {
		s.Increment([]byte("k"))
	}
	s.Reset()
	if s.Estimate([]byte("k")) > 10 {
		t.Fatal("reset 后应降低")
	}
}

func TestContains(t *testing.T) {
	c := New(16)
	c.Put("a", 1)
	if !c.Contains("a") {
		t.Fatal("contains")
	}
	if c.Contains("missing") {
		t.Fatal("missing")
	}
}

func TestPeek(t *testing.T) {
	c := New(16)
	c.Put("a", 1)
	v, ok := c.Peek("a")
	if !ok || v.(int) != 1 {
		t.Fatal("peek", v, ok)
	}
}

func TestDelete(t *testing.T) {
	c := New(16)
	c.Put("a", 1)
	if !c.Delete("a") {
		t.Fatal("delete")
	}
	if c.Delete("a") {
		t.Fatal("double delete")
	}
}

func TestStats(t *testing.T) {
	c := New(4)
	c.Put("a", 1)
	c.Get("a")
	c.Get("missing")
	// 触发 evict
	for i := 0; i < 10; i++ {
		c.Put(string(rune('b'+i)), i)
	}
	st := c.Stats()
	if st.Hits == 0 || st.Misses == 0 {
		t.Fatalf("stats: %+v", st)
	}
	if st.Evictions == 0 {
		t.Fatal("evictions 应 > 0")
	}
}

func TestClear(t *testing.T) {
	c := New(4)
	for i := 0; i < 10; i++ {
		c.Put(string(rune('a'+i)), i)
	}
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}

func TestCap(t *testing.T) {
	c := New(8)
	if c.Cap() != 8 {
		t.Fatal("cap")
	}
}
