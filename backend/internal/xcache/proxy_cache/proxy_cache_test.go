package proxy_cache

import (
	"errors"
	"testing"
)

type memLocal struct {
	m map[string]string
}

func newMemLocal() *memLocal { return &memLocal{m: make(map[string]string)} }
func (l *memLocal) Get(k string) (string, bool) { v, ok := l.m[k]; return v, ok }
func (l *memLocal) Put(k, v string)              { l.m[k] = v }

type fakeRemote struct {
	m map[string]string
}

func newFakeRemote() *fakeRemote { return &fakeRemote{m: make(map[string]string)} }
func (r *fakeRemote) Get(k string) (string, error) {
	v, ok := r.m[k]
	if !ok {
		return "", errors.New("miss")
	}
	return v, nil
}
func (r *fakeRemote) Put(k, v string) error { r.m[k] = v; return nil }

func TestLocalHit(t *testing.T) {
	l := newMemLocal()
	rm := newFakeRemote()
	l.Put("a", "1")
	p := New(l, rm)
	v, ok := p.Get("a")
	if !ok || v != "1" {
		t.Fatal("hit", v)
	}
	hits, _ := p.Stats()
	if hits != 1 {
		t.Fatal("hits", hits)
	}
}

func TestRemoteBackfill(t *testing.T) {
	l := newMemLocal()
	rm := newFakeRemote()
	rm.Put("a", "1")
	p := New(l, rm)
	v, _ := p.Get("a")
	if v != "1" {
		t.Fatal("miss")
	}
	if _, ok := l.Get("a"); !ok {
		t.Fatal("未回填")
	}
}

func TestRemoteErr(t *testing.T) {
	l := newMemLocal()
	rm := newFakeRemote()
	p := New(l, rm)
	if _, ok := p.Get("x"); ok {
		t.Fatal("err")
	}
}

func TestPut(t *testing.T) {
	l := newMemLocal()
	rm := newFakeRemote()
	p := New(l, rm)
	if err := p.Put("a", "x"); err != nil {
		t.Fatal(err)
	}
	if l.m["a"] != "x" || rm.m["a"] != "x" {
		t.Fatal("put")
	}
}
