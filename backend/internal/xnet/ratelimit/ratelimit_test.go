package ratelimit

import (
	"testing"
	"time"
)

func TestAllow_Burst(t *testing.T) {
	l := New(5, 1)
	for i := 0; i < 5; i++ {
		if !l.Allow() {
			t.Fatal("突发内应允许")
		}
	}
	if l.Allow() {
		t.Fatal("超突发应拒")
	}
}

func TestRefill(t *testing.T) {
	l := New(2, 10) // 10/s
	for i := 0; i < 2; i++ {
		l.Allow()
	}
	if l.Allow() {
		t.Fatal("应拒")
	}
	time.Sleep(200 * time.Millisecond)
	if !l.Allow() {
		t.Fatal("补充后应允许")
	}
}

func TestAllowN(t *testing.T) {
	l := New(5, 1)
	if !l.AllowN(3) {
		t.Fatal("3 应允许")
	}
	if !l.AllowN(2) {
		t.Fatal("2 应允许")
	}
	if l.AllowN(1) {
		t.Fatal("1 应拒")
	}
}

func TestWait(t *testing.T) {
	l := New(1, 20)
	l.Allow()
	done := make(chan struct{})
	go func() {
		l.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("应能等到令牌")
	}
}

func TestTokens(t *testing.T) {
	l := New(3, 1)
	tk := l.Tokens()
	if tk < 2 || tk > 3 {
		t.Fatal("tokens:", tk)
	}
}

func TestSetRate(t *testing.T) {
	l := New(1, 1)
	l.SetRate(100)
	time.Sleep(100 * time.Millisecond)
	if l.Tokens() < 1 {
		t.Fatal("setRate")
	}
}
