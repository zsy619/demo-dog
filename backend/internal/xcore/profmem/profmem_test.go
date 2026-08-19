package profmem

import (
	"testing"
	"time"
)

func TestCapture(t *testing.T) {
	s := Capture()
	if s.SysBytes == 0 {
		t.Fatal("sys")
	}
}

func TestDelta(t *testing.T) {
	a := Capture()
	time.Sleep(10 * time.Millisecond)
	b := Capture()
	d := Delta(a, b)
	if d.Span < time.Millisecond {
		t.Fatal("span")
	}
}

func TestAllocRate(t *testing.T) {
	d := DeltaInfo{Total: 1000, Span: time.Second}
	if d.AllocRate() != 1000 {
		t.Fatal("rate")
	}
}

func TestTrack(t *testing.T) {
	tr := NewTrack(20 * time.Millisecond)
	time.Sleep(80 * time.Millisecond)
	tr.Stop()
	_, on := tr.RateSnapshot()
	if !on {
		t.Fatal("track")
	}
}

func TestForceGC(t *testing.T) {
	ForceGC()
}
