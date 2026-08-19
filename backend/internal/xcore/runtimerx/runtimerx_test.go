package runtimerx

import (
	"sync/atomic"
	"testing"
)

func TestCapture(t *testing.T) {
	i := Capture()
	if i.NumCPU < 1 {
		t.Fatal("cpu")
	}
	if i.GoVersion == "" {
		t.Fatal("go")
	}
}

func TestSetMaxProcs(t *testing.T) {
	SetMaxProcs(2)
	if Capture().GOMAXPROCS != 2 {
		t.Fatal("procs")
	}
}

func TestNumGoroutine(t *testing.T) {
	if NumGoroutine() < 1 {
		t.Fatal("numg")
	}
}

func TestParallel(t *testing.T) {
	var sum atomic.Int64
	Parallel(100, func(s, e int) {
		for i := s; i < e; i++ {
			sum.Add(1)
		}
	})
	if sum.Load() != 100 {
		t.Fatal("sum", sum.Load())
	}
}

func TestParallel_Zero(t *testing.T) {
	Parallel(0, func(_, _ int) {})
}
