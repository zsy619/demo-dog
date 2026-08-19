package singleflight

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingle(t *testing.T) {
	g := New()
	v, _, shared := g.Do("k", func() (any, error) { return 1, nil })
	if v.(int) != 1 || shared {
		t.Fatal("首次应独占")
	}
}

func TestShared(t *testing.T) {
	g := New()
	calls := atomic.Int32{}
	fn := func() (any, error) {
		calls.Add(1)
		time.Sleep(30 * time.Millisecond)
		return 42, nil
	}
	var wg sync.WaitGroup
	results := make([]int, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, _, _ := g.Do("k", fn)
			results[i] = v.(int)
		}(i)
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("应只调用一次")
	}
	for _, r := range results {
		if r != 42 {
			t.Fatal("应共享")
		}
	}
}

func TestDifferentKeys(t *testing.T) {
	g := New()
	calls := atomic.Int32{}
	fn := func() (any, error) { calls.Add(1); return 1, nil }
	g.Do("a", fn)
	g.Do("b", fn)
	if calls.Load() != 2 {
		t.Fatal("不同 key 应独立")
	}
}

func TestForget(t *testing.T) {
	g := New()
	done := make(chan struct{})
	go func() {
		g.Do("k", func() (any, error) {
			<-done
			return 1, nil
		})
	}()
	time.Sleep(10 * time.Millisecond)
	if g.InFlight() != 1 {
		t.Fatal("应有飞行中")
	}
	g.Forget("k")
	close(done)
}

func TestInFlight(t *testing.T) {
	g := New()
	if g.InFlight() != 0 {
		t.Fatal("初始应 0")
	}
}
