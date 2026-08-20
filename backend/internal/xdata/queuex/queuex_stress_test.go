package queuex

import (
	"sync"
	"testing"
)

func TestPushPopStress(t *testing.T) {
	q := New[int](10)
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			q.Push(i)
		}(i)
		go func() {
			defer wg.Done()
			q.Pop()
		}()
	}
	wg.Wait()
}

func TestPushWhenFull(t *testing.T) {
	q := New[int](2)
	q.Push(1)
	q.Push(2)
	old, ok := q.Push(3)
	if !ok || old != 1 {
		t.Fatalf("Push(3) when full 应覆盖 1 返回 (1,true)，得到 (%d,%v)", old, ok)
	}
}
