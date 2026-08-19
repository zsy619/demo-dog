package shardkv

import (
	"bytes"
	"sync"
	"testing"
)

func TestPutGet(t *testing.T) {
	s := New(8)
	s.Put("a", []byte("1"))
	v, ok := s.Get("a")
	if !ok || !bytes.Equal(v, []byte("1")) {
		t.Fatal("get")
	}
}

func TestDelete(t *testing.T) {
	s := New(8)
	s.Put("a", []byte("1"))
	s.Delete("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("del")
	}
}

func TestLen(t *testing.T) {
	s := New(8)
	s.Put("a", nil)
	s.Put("b", nil)
	if s.Len() != 2 {
		t.Fatal("len")
	}
}

func TestShards(t *testing.T) {
	s := New(4)
	if s.Shards() != 4 {
		t.Fatal("shards")
	}
}

func TestConcurrent(t *testing.T) {
	s := New(8)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			s.Put("k", []byte{byte(i)})
		}(i)
		go func() {
			defer wg.Done()
			s.Get("k")
		}()
	}
	wg.Wait()
}
