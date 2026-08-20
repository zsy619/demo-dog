package router

import (
	"errors"
	"testing"
)

func noop(w, r any) {}

func TestStatic(t *testing.T) {
	rt := New()
	if err := rt.Register(GET, "/users", noop); err != nil {
		t.Fatal(err)
	}
	h, _, err := rt.Match(GET, "/users")
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("no handler")
	}
}

func TestParam(t *testing.T) {
	rt := New()
	rt.Register(GET, "/users/:id", noop)
	_, params, err := rt.Match(GET, "/users/123")
	if err != nil {
		t.Fatal(err)
	}
	if params["id"] != "123" {
		t.Fatal("param")
	}
}

func TestMultiParam(t *testing.T) {
	rt := New()
	rt.Register(GET, "/users/:uid/posts/:pid", noop)
	_, params, err := rt.Match(GET, "/users/u1/posts/p2")
	if err != nil {
		t.Fatal(err)
	}
	if params["uid"] != "u1" || params["pid"] != "p2" {
		t.Fatal("multi")
	}
}

func TestMethodMismatch(t *testing.T) {
	rt := New()
	rt.Register(GET, "/x", noop)
	if _, _, err := rt.Match(POST, "/x"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestNotFound(t *testing.T) {
	rt := New()
	if _, _, err := rt.Match(GET, "/missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestRoot(t *testing.T) {
	rt := New()
	rt.Register(GET, "/", noop)
	h, _, err := rt.Match(GET, "/")
	if err != nil || h == nil {
		t.Fatal(err)
	}
}

func TestMultiMethod(t *testing.T) {
	rt := New()
	h1 := func(w, r any) {}
	h2 := func(w, r any) {}
	rt.Register(GET, "/x", h1)
	rt.Register(POST, "/x", h2)
	g, _, _ := rt.Match(GET, "/x")
	p, _, _ := rt.Match(POST, "/x")
	if g == nil || p == nil {
		t.Fatal("multi-method")
	}
}

func TestStaticAndParam(t *testing.T) {
	rt := New()
	rt.Register(GET, "/users/me", noop)
	rt.Register(GET, "/users/:id", noop)
	_, params, _ := rt.Match(GET, "/users/me")
	if _, ok := params["id"]; ok {
		t.Fatal("static should not match param")
	}
	_, params, _ = rt.Match(GET, "/users/abc")
	if params["id"] != "abc" {
		t.Fatal("param")
	}
}

func TestCount(t *testing.T) {
	rt := New()
	rt.Register(GET, "/a", noop)
	rt.Register(POST, "/a", noop)
	rt.Register(GET, "/b/:x", noop)
	if rt.Count() != 3 {
		t.Fatal("count")
	}
}

func TestBadPattern(t *testing.T) {
	rt := New()
	if err := rt.Register(GET, "x", noop); !errors.Is(err, ErrBadPattern) {
		t.Fatal(err)
	}
}
