package ratelimit

import (
	"testing"
	"time"
)

func TestAllow(t *testing.T) {
	l := New(2, 1)
	if !l.Allow() {
		t.Fatal("first")
	}
	if !l.Allow() {
		t.Fatal("second")
	}
	if l.Allow() {
		t.Fatal("third 应拒")
	}
}

func TestAllowN(t *testing.T) {
	l := New(10, 1)
	if !l.AllowN(5) {
		t.Fatal("n")
	}
}

func TestRefill(t *testing.T) {
	l := New(1, 10)
	l.Allow()
	time.Sleep(200 * time.Millisecond)
	if !l.Allow() {
		t.Fatal("refill")
	}
}

func TestWait(t *testing.T) {
	l := New(1, 20)
	if !l.Wait(time.Second) {
		t.Fatal("wait")
	}
}

func TestWait_Timeout(t *testing.T) {
	l := New(1, 0)
	l.Allow()
	if l.Wait(30 * time.Millisecond) {
		t.Fatal("timeout 应 false")
	}
}

func TestTokens(t *testing.T) {
	l := New(5, 1)
	if l.Tokens() < 4 {
		t.Fatal("tk")
	}
}

func TestRate(t *testing.T) {
	l := New(1, 2)
	if l.Rate() != 2 {
		t.Fatal("rate")
	}
}
