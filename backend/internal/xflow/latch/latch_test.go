package latch

import (
	"testing"
	"time"
)

func TestCountDown(t *testing.T) {
	l := NewCountDown(2)
	done := make(chan struct{})
	go func() {
		l.Wait()
		close(done)
	}()
	l.CountDown()
	l.CountDown()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestStartLatch(t *testing.T) {
	l := NewStartLatch()
	done := make(chan struct{})
	go func() {
		l.Wait()
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	l.Release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestStartLatch_DoubleRelease(t *testing.T) {
	l := NewStartLatch()
	l.Release()
	l.Release() // 不应 panic
}
