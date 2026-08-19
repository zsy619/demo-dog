package coalesce

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoalesce_Single(t *testing.T) {
	g := NewGroup()
	v, _, first := g.Do("k", func() (any, error) { return 1, nil })
	if v.(int) != 1 || !first {
		t.Fatal("首次")
	}
}

func TestCoalesce_Shared(t *testing.T) {
	g := NewGroup()
	calls := atomic.Int32{}
	fn := func() (any, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
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

func TestCoalesce_DifferentKeys(t *testing.T) {
	g := NewGroup()
	calls := atomic.Int32{}
	fn := func() (any, error) { calls.Add(1); return 1, nil }
	g.Do("a", fn)
	g.Do("b", fn)
	if calls.Load() != 2 {
		t.Fatal("不同 key 应独立")
	}
}

func TestForget(t *testing.T) {
	g := NewGroup()
	g.Forget("x")
	if g.Len() != 0 {
		t.Fatal("forget")
	}
}

func TestLen(t *testing.T) {
	g := NewGroup()
	done := make(chan struct{})
	go func() {
		g.Do("k", func() (any, error) {
			<-done
			return 1, nil
		})
	}()
	time.Sleep(10 * time.Millisecond)
	if g.Len() != 1 {
		t.Fatal("应有飞行中")
	}
	close(done)
}
