package inspectx

import "testing"

func TestCapture(t *testing.T) {
	i := Capture()
	if i.NumCPU == 0 || i.GOMAXPROCS == 0 {
		t.Fatal("cpu")
	}
}

func TestProbe(t *testing.T) {
	p := NewProbe()
	snap := p.Snapshot()
	if snap.NumCPU == 0 {
		t.Fatal("snap")
	}
	p.Refresh()
	if p.Snapshot().NumCPU != snap.NumCPU {
		t.Fatal("refresh")
	}
}

func TestNil(t *testing.T) {
	// nil probe Snapshot 会 panic，确保该代码路径不被依赖
	defer func() {
		_ = recover()
	}()
	var p *Probe
	p.Snapshot()
}
