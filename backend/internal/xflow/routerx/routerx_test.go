package routerx

import "testing"

func TestStatic(t *testing.T) {
	r := New()
	called := false
	r.Add("/foo", func(_ map[string]string) { called = true })
	h, _ := r.Match("/foo")
	if h == nil {
		t.Fatal("miss")
	}
	h(nil)
	if !called {
		t.Fatal("call")
	}
}

func TestParam(t *testing.T) {
	r := New()
	var got string
	r.Add("/users/:id", func(p map[string]string) { got = p["id"] })
	h, params := r.Match("/users/42")
	if h == nil {
		t.Fatal("miss")
	}
	h(params)
	if got != "42" {
		t.Fatal("param", got)
	}
}

func TestMiss(t *testing.T) {
	r := New()
	r.Add("/a", func(_ map[string]string) {})
	if h, _ := r.Match("/b"); h != nil {
		t.Fatal("miss")
	}
}

func TestStar(t *testing.T) {
	r := New()
	var got string
	r.Add("/static/*", func(p map[string]string) { got = "star" })
	h, params := r.Match("/static/foo/bar")
	if h == nil {
		t.Fatal("star miss")
	}
	h(params)
	if got != "star" {
		t.Fatal("star")
	}
}
