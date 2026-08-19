package proxy

import "testing"

func TestFromEnv(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy:8080")
	u := FromEnv()
	if u == nil || u.Host != "proxy:8080" {
		t.Fatal("env")
	}
}

func TestFromEnv_Empty(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")
	if u := FromEnv(); u != nil {
		t.Fatal("empty", u)
	}
}

func TestNoProxy(t *testing.T) {
	t.Setenv("NO_PROXY", "*.example.com")
	if !NoProxy("a.example.com") {
		t.Fatal("wild")
	}
	if NoProxy("other.com") {
		t.Fatal("miss")
	}
}

func TestNoProxy_Star(t *testing.T) {
	t.Setenv("NO_PROXY", "*")
	if !NoProxy("anything.com") {
		t.Fatal("star")
	}
}

func TestResolve(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://p:1")
	t.Setenv("HTTPS_PROXY", "http://q:2")
	t.Setenv("NO_PROXY", "")
	if Resolve("http://x.com") == nil {
		t.Fatal("http")
	}
	if Resolve("https://x.com") == nil {
		t.Fatal("https")
	}
	t.Setenv("NO_PROXY", "*.x.com")
	if Resolve("http://a.x.com") != nil {
		t.Fatal("noproxy")
	}
}
