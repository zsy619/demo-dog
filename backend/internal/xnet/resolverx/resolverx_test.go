package resolverx

import (
	"context"
	"testing"
	"time"
)

func TestLookupLocalhost(t *testing.T) {
	r := New(time.Minute)
	ips, err := r.LookupIP(context.Background(), "localhost")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) == 0 {
		t.Fatal("empty")
	}
}

func TestCache(t *testing.T) {
	r := New(time.Hour)
	_, _ = r.LookupIP(context.Background(), "localhost")
	if r.Len() != 1 {
		t.Fatal("cache size", r.Len())
	}
	_, _ = r.LookupIP(context.Background(), "localhost")
	if r.Len() != 1 {
		t.Fatal("should still be 1")
	}
}

func TestInvalidate(t *testing.T) {
	r := New(time.Hour)
	_, _ = r.LookupIP(context.Background(), "localhost")
	r.Invalidate("localhost")
	if r.Len() != 0 {
		t.Fatal("inv")
	}
}

func TestClear(t *testing.T) {
	r := New(time.Hour)
	_, _ = r.LookupIP(context.Background(), "localhost")
	r.Clear()
	if r.Len() != 0 {
		t.Fatal("clr")
	}
}


