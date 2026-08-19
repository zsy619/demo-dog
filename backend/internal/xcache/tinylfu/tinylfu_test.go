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
