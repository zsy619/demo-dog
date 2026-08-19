package healthprobe

import (
	"net"
	"testing"
	"time"
)

func startListener(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

func TestAdd(t *testing.T) {
	p := New(time.Hour, time.Second)
	p.Add("x:0")
	if len(p.Snapshot()) != 1 {
		t.Fatal("add")
	}
}

func TestProbe_Up(t *testing.T) {
	addr := startListener(t)
	p := New(time.Hour, time.Second)
	p.Add(addr)
	p.probeOne(addr)
	if p.Status(addr) != StatusUp {
		t.Fatal("应 up")
	}
}

func TestProbe_Down(t *testing.T) {
	p := New(time.Hour, 100*time.Millisecond)
	p.Add("127.0.0.1:1")
	p.probeOne("127.0.0.1:1")
	if p.Status("127.0.0.1:1") != StatusDown {
		t.Fatal("应 down")
	}
}

func TestStatus_Unknown(t *testing.T) {
	p := New(time.Hour, time.Second)
	if p.Status("x:0") != StatusUnknown {
		t.Fatal("unknown")
	}
}

func TestStartStop(t *testing.T) {
	p := New(50*time.Millisecond, 50*time.Millisecond)
	p.Start()
	time.Sleep(20 * time.Millisecond)
	p.Stop()
}
