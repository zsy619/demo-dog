package barrier

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBarrier(t *testing.T) {
	b := New(3)
	var done atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Wait()
			done.Add(1)
		}()
	}
	wg.Wait()
	if done.Load() != 3 {
		t.Fatal("done", done.Load())
	}
}

func TestRelease(t *testing.T) {
	b := New(2)
	go func() {
		b.Wait()
		b.Wait()
	}()
	time.Sleep(10 * time.Millisecond)
	go b.Release()
	time.Sleep(10 * time.Millisecond)
	if b.Waiting() < 0 {
		t.Fatal("waiting")
	}
}

func TestReset(t *testing.T) {
	b := New(2)
	b.Reset()
	if b.Waiting() != 0 {
		t.Fatal("reset")
	}
}

func TestWaiting(t *testing.T) {
	b := New(10)
	if b.Waiting() != 0 {
		t.Fatal("empty")
	}
}
