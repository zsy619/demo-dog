package tcphealth

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func listenLoop(t *testing.T) (string, func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }
}

func TestCheck_Healthy(t *testing.T) {
	addr, stop := listenLoop(t)
	defer stop()
	m := New([]string{addr}, time.Second)
	m.Check(context.Background())
	snap := m.Snapshot()
	if !snap[addr].Healthy {
		t.Fatal("should be healthy")
	}
}

func TestCheck_Unhealthy(t *testing.T) {
	m := New([]string{"127.0.0.1:1"}, 100*time.Millisecond)
	m.Check(context.Background())
	snap := m.Snapshot()
	if snap["127.0.0.1:1"].Healthy {
		t.Fatal("should be unhealthy")
	}
}

func TestAdd(t *testing.T) {
	m := New(nil, time.Second)
	m.Add("a")
	if len(m.Snapshot()) != 1 {
		t.Fatal("add")
	}
}

func TestHealthiest(t *testing.T) {
	addr, stop := listenLoop(t)
	defer stop()
	m := New([]string{addr, "127.0.0.1:1"}, time.Second)
	m.Check(context.Background())
	best, err := m.Healthiest()
	if err != nil || best != addr {
		t.Fatal("best")
	}
}

func TestHealthiest_None(t *testing.T) {
	m := New([]string{"127.0.0.1:1"}, 100*time.Millisecond)
	if _, err := m.Healthiest(); !errors.Is(err, ErrNoAddr) {
		t.Fatal(err)
	}
}

func TestStats(t *testing.T) {
	addr, stop := listenLoop(t)
	defer stop()
	m := New([]string{addr, "127.0.0.1:1"}, time.Second)
	m.Check(context.Background())
	s := m.Stats()
	if s.Attempted != 2 || s.Succeeded != 1 || s.Failed != 1 {
		t.Fatal("stats")
	}
}

func TestCheck_AdaptiveTimeout(t *testing.T) {
	addr, stop := listenLoop(t)
	defer stop()
	m := New([]string{addr}, time.Second)
	m.Check(context.Background())
	first := m.Snapshot()[addr]
	if !first.Healthy {
		t.Fatal("first")
	}
	m.Check(context.Background())
	second := m.Snapshot()[addr]
	if second.Latency == 0 {
		t.Fatal("second latency")
	}
}
