package segment

import (
	"sync"
	"testing"
)

func TestPutGet(t *testing.T) {
	c := New(4, 8)
	c.Put("a", 1)
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatal("get")
	}
}

func TestEviction(t *testing.T) {
	c := New(1, 2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // 同段单 cap=2
	if c.Len() != 2 {
		t.Fatal("len")
	}
}

func TestDelete(t *testing.T) {
	c := New(4, 4)
	c.Put("a", 1)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("应删")
	}
}

func TestUpdate(t *testing.T) {
	c := New(4, 4)
	c.Put("a", 1)
	c.Put("a", 2)
	v, _ := c.Get("a")
	if v.(int) != 2 {
		t.Fatal("update")
	}
}

func TestConcurrent(t *testing.T) {
	c := New(8, 64)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.Put("k", i)
		}(i)
		go func(i int) {
			defer wg.Done()
			c.Get("k")
		}(i)
	}
	wg.Wait()
}

func TestClear(t *testing.T) {
	c := New(4, 4)
	c.Put("a", 1)
	c.Clear()
	if c.Len() != 0 {
		t.Fatal("clear")
	}
}
