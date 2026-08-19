package iplimit

import (
	"testing"
	"time"
)

func TestAllow(t *testing.T) {
	l := New(time.Minute, 3)
	defer l.Close()
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatal("前 3 次应允许")
		}
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("第 4 次应拒")
	}
}

func TestDifferentIPs(t *testing.T) {
	l := New(time.Minute, 1)
	defer l.Close()
	l.Allow("a")
	if !l.Allow("b") {
		t.Fatal("不同 IP 应独立")
	}
}

func TestReset(t *testing.T) {
	l := New(time.Minute, 1)
	defer l.Close()
	l.Allow("a")
	l.Reset("a")
	if !l.Allow("a") {
		t.Fatal("reset 后应允许")
	}
}

func TestCount(t *testing.T) {
	l := New(time.Minute, 10)
	defer l.Close()
	l.Allow("a")
	l.Allow("a")
	if l.Count("a") != 2 {
		t.Fatal("count")
	}
}

func TestWindow(t *testing.T) {
	l := New(20*time.Millisecond, 1)
	defer l.Close()
	l.Allow("a")
	time.Sleep(40 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("窗口外应允许")
	}
}
