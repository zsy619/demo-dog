package promise

import (
	"errors"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	p := New[int]()
	p.Resolve(42)
	v, _ := p.Await()
	if v != 42 {
		t.Fatal("v")
	}
}

func TestReject(t *testing.T) {
	p := New[int]()
	p.Reject(errors.New("x"))
	if _, err := p.Await(); err == nil {
		t.Fatal("err")
	}
}

func TestOnce(t *testing.T) {
	p := New[int]()
	p.Resolve(1)
	p.Resolve(2)
	v, _ := p.Await()
	if v != 1 {
		t.Fatal("once")
	}
}

func TestIsDone(t *testing.T) {
	p := New[int]()
	if p.IsDone() {
		t.Fatal("应未完成")
	}
	p.Resolve(1)
	if !p.IsDone() {
		t.Fatal("应完成")
	}
}

func TestRun(t *testing.T) {
	p := Run(func() (int, error) { time.Sleep(20 * time.Millisecond); return 7, nil })
	v, err := p.Await()
	if err != nil || v != 7 {
		t.Fatal("run")
	}
}

func TestAll(t *testing.T) {
	a := New[int]()
	b := New[int]()
	go func() { a.Resolve(1) }()
	go func() { b.Resolve(2) }()
	vs, err := All(a, b)
	if err != nil || len(vs) != 2 || vs[0] != 1 || vs[1] != 2 {
		t.Fatal("all")
	}
}
