package router

import "testing"

func TestMatch(t *testing.T) {
	got := ""
	r := New()
	r.Add("/users/:id/posts", func(p map[string]string) { got = p["id"] })
	params, ok := r.Match("/users/42/posts")
	if !ok || params["id"] != "42" {
		t.Fatal("match", params)
	}
	_ = got
}

func TestMiss(t *testing.T) {
	r := New()
	r.Add("/a", func(_ map[string]string) {})
	if _, ok := r.Match("/b"); ok {
		t.Fatal("miss")
	}
}

func TestRoot(t *testing.T) {
	r := New()
	r.Add("/", func(_ map[string]string) {})
	if _, ok := r.Match("/"); !ok {
		t.Fatal("root")
	}
}

func TestNoParam(t *testing.T) {
	r := New()
	r.Add("/x/y", func(_ map[string]string) {})
	params, ok := r.Match("/x/y")
	if !ok || params != nil && len(params) != 0 {
		t.Fatal("plain", params)
	}
}

func TestMultipleParams(t *testing.T) {
	r := New()
	r.Add("/:a/:b/:c", func(_ map[string]string) {})
	params, _ := r.Match("/1/2/3")
	if params["a"] != "1" || params["b"] != "2" || params["c"] != "3" {
		t.Fatal("multi", params)
	}
}
