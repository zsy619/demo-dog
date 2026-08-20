package qcache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStampede(t *testing.T) {
	var calls atomic.Int32
	c := New(func(k string) (int, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return len(k), nil
	})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get("key")
		}()
	}
	wg.Wait()
	if calls.Load() > 5 {
		t.Fatalf("callers 应被合并，实际 %d 次", calls.Load())
	}
}
