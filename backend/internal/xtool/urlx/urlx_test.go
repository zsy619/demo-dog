package urlx

import (
	"net/url"
	"testing"
)

func TestQueryValues(t *testing.T) {
	v := QueryValues("a=1&b=2")
	if v.Get("a") != "1" || v.Get("b") != "2" {
		t.Fatal("qv")
	}
}

func TestEncode(t *testing.T) {
	v := url.Values{"a": []string{"1"}}
	if Encode(v) != "a=1" {
		t.Fatal("enc")
	}
}

func TestJoinPath(t *testing.T) {
	if got := JoinPath("a/", "b", "c"); got != "a/b/c" {
		t.Fatal("join", got)
	}
	if got := JoinPath("a", "/b/", "c"); got != "a/b/c" {
		t.Fatal("join2", got)
	}
}

func TestGet(t *testing.T) {
	v := url.Values{}
	if Get(v, "x", "d") != "d" {
		t.Fatal("def")
	}
	v.Set("x", "1")
	if Get(v, "x", "d") != "1" {
		t.Fatal("val")
	}
}

func TestFirst(t *testing.T) {
	v := url.Values{}
	if First(v, "x", "d") != "d" {
		t.Fatal("def")
	}
	v.Add("x", "a")
	v.Add("x", "b")
	if First(v, "x", "d") != "a" {
		t.Fatal("first")
	}
}

func TestIsAbsolute(t *testing.T) {
	if !IsAbsolute("http://x.com") {
		t.Fatal("abs")
	}
	if IsAbsolute("/x") {
		t.Fatal("rel")
	}
}

func TestHostPort(t *testing.T) {
	h, p, ok := HostPort("http://example.com:8080/x")
	if !ok || h != "example.com" || p != 8080 {
		t.Fatal("hostport")
	}
}

func TestMerge(t *testing.T) {
	a := url.Values{"x": []string{"1"}}
	b := url.Values{"y": []string{"2"}}
	m := Merge(a, b)
	if m.Get("x") != "1" || m.Get("y") != "2" {
		t.Fatal("merge")
	}
}
