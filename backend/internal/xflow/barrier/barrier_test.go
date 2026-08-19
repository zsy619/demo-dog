package barrier

import (
	"context"
	"testing"
	"time"
)

func TestCountBarrier_Reached(t *testing.T) {
	b := NewCount(2)
	b.Done()
	if b.Wait(context.Background()) {
		t.Fatal("未达 target")
	}
	b.Done()
	if !b.Wait(context.Background()) {
		t.Fatal("应达 target")
	}
}

func TestCountBarrier_Cancel(t *testing.T) {
	b := NewCount(5)
	b.Cancel()
	if b.Wait(context.Background()) {
		t.Fatal("cancel 应 false")
	}
}

func TestCountBarrier_Context(t *testing.T) {
	b := NewCount(5)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if b.Wait(ctx) {
		t.Fatal("应取消")
	}
}

func TestCountBarrier_DoneCount(t *testing.T) {
	b := NewCount(3)
	b.Done()
	b.Done()
	if b.DoneCount() != 2 {
		t.Fatal("count")
	}
}

func TestTimeBarrier_Wait(t *testing.T) {
	b := NewTime(time.Now().Add(30 * time.Millisecond))
	start := time.Now()
	if !b.Wait() {
		t.Fatal("应达到")
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Fatal("未等到时间")
	}
}

func TestTimeBarrier_Cancel(t *testing.T) {
	b := NewTime(time.Now().Add(time.Second))
	b.Cancel()
	if b.Wait() {
		t.Fatal("应取消")
	}
}
