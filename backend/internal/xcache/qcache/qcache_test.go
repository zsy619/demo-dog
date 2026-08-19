package qcache

import (
	"errors"
	"testing"
)

func TestCacheHit(t *testing.T) {
	var calls int
	c := New(func(k string) (int, error) { calls++; return len(k), nil })
	v, _ := c.Get("ab")
	v2, _ := c.Get("ab")
	if v != 2 || v2 != 2 {
		t.Fatal("v", v, v2)
	}
	if calls != 1 {
		t.Fatal("calls", calls)
	}
}

func TestCacheMiss(t *testing.T) {
	c := New(func(k int) (int, error) { return k * 2, nil })
	v, _ := c.Get(3)
	if v != 6 {
		t.Fatal("miss", v)
	}
	if c.Len() != 1 {
		t.Fatal("len")
	}
}

func TestLoaderErr(t *testing.T) {
	myErr := errors.New("x")
	c := New(func(k string) (string, error) { return "", myErr })
	if _, err := c.Get("a"); err == nil {
		t.Fatal("err")
	}
}

func TestInvalidate(t *testing.T) {
	c := New(func(k int) (int, error) { return k, nil })
	c.Get(1)
	c.Invalidate(1)
	if c.Len() != 0 {
		t.Fatal("inv")
	}
}
