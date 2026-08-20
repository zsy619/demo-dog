package debug

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newReq() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/debug/info", nil)
}

func TestGate_Open_NoToken(t *testing.T) {
	g := NewGate("")
	if err := g.Allow(newReq()); err != nil {
		t.Fatal(err)
	}
	if !g.Stats().Open {
		t.Fatal("should be open")
	}
}

func TestGate_TokenAccept(t *testing.T) {
	g := NewGate("secret")
	r := newReq()
	r.Header.Set("X-Debug-Token", "secret")
	if err := g.Allow(r); err != nil {
		t.Fatal(err)
	}
	if g.Stats().Hits != 1 {
		t.Fatal("hits")
	}
}

func TestGate_TokenReject(t *testing.T) {
	g := NewGate("secret")
	r := newReq()
	r.Header.Set("X-Debug-Token", "wrong")
	if err := g.Allow(r); err == nil {
		t.Fatal("expected error")
	}
	if g.Stats().Reject != 1 {
		t.Fatal("reject counter")
	}
}

func TestGate_NoHeader(t *testing.T) {
	g := NewGate("secret")
	if err := g.Allow(newReq()); err == nil {
		t.Fatal("expected error")
	}
}

func TestHandler_RejectsWithoutToken(t *testing.T) {
	g := NewGate("secret")
	h := Handler(g, Version{Service: "x", GoVersion: "go1.26"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, newReq())
	if rr.Code != 401 {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestHandler_AcceptsWithToken(t *testing.T) {
	g := NewGate("secret")
	h := Handler(g, Version{Service: "x", GoVersion: "go1.26"})
	r := newReq()
	r.Header.Set("X-Debug-Token", "secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	if rr.Code != 200 {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}
	if !contains(rr.Body.String(), "go_version=go1.26") {
		t.Fatal("expected go_version in info")
	}
}

func TestHandler_Version(t *testing.T) {
	g := NewGate("")
	h := Handler(g, Version{Service: "demo-dog", BuildSHA: "abc", GoVersion: "go1.26"})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	h.ServeHTTP(rr, r)
	if rr.Code != 200 {
		t.Fatalf("status: %d", rr.Code)
	}
	if !contains(rr.Body.String(), `"service":"demo-dog"`) {
		t.Fatal("service")
	}
}

func TestHandler_Stack(t *testing.T) {
	g := NewGate("")
	h := Handler(g, Version{})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/debug/stack", nil)
	h.ServeHTTP(rr, r)
	if rr.Code != 200 {
		t.Fatalf("status: %d", rr.Code)
	}
	if rr.Body.Len() < 100 {
		t.Fatal("expected stack dump")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
