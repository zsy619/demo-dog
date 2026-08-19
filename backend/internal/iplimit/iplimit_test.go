package iplimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newLimiter() *Limiter {
	return New(2, 2).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestAllow_Burst(t *testing.T) {
	l := newLimiter()
	for i := 0; i < 2; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("burst %d denied", i)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("third should be denied")
	}
}

func TestAllow_PerIPIsolation(t *testing.T) {
	l := newLimiter()
	l.Allow("1.1.1.1")
	l.Allow("1.1.1.1")
	if !l.Allow("2.2.2.2") {
		t.Fatal("different IP should be allowed")
	}
}

func TestAllow_Refill(t *testing.T) {
	now := time.Unix(1700000000, 0)
	l := New(10, 10).WithTime(func() time.Time { return now })
	for i := 0; i < 10; i++ {
		l.Allow("1.2.3.4")
	}
	now = now.Add(time.Second)
	if !l.Allow("1.2.3.4") {
		t.Fatal("refill should allow")
	}
}

func TestStats(t *testing.T) {
	l := newLimiter()
	l.Allow("1.2.3.4")
	l.Allow("1.2.3.4")
	l.Allow("1.2.3.4")
	s := l.Stats()
	if s.Accepted != 2 || s.Rejected != 1 || s.IPs != 1 {
		t.Fatalf("stats: %+v", s)
	}
}

func TestCleanup(t *testing.T) {
	now := time.Unix(1700000000, 0)
	l := newLimiter().WithTime(func() time.Time { return now })
	l.Allow("1.2.3.4")
	now = now.Add(time.Hour)
	n := l.Cleanup(30 * time.Second)
	if n != 1 {
		t.Fatal("cleanup")
	}
}

func TestMiddleware_Allows(t *testing.T) {
	l := newLimiter()
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "1.2.3.4:5000"
	h.ServeHTTP(rr, r)
	if rr.Code != 200 {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestMiddleware_Blocks(t *testing.T) {
	l := newLimiter()
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "1.2.3.4:5000"
		h.ServeHTTP(rr, r)
	}
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "1.2.3.4:5000"
	h.ServeHTTP(rr, r)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestMiddleware_XForwardedFor(t *testing.T) {
	l := newLimiter()
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", "9.9.9.9")
		h.ServeHTTP(rr, r)
	}
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "9.9.9.9")
	h.ServeHTTP(rr, r)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		remote string
		want   string
	}{
		{"1.2.3.4:8080", "1.2.3.4"},
		{"1.2.3.4", "1.2.3.4"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = c.remote
		if got := clientIP(r); got != c.want {
			t.Fatalf("got %s want %s", got, c.want)
		}
	}
}
