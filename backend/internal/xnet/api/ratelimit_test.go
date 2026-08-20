package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	ip := "1.2.3.4"
	for i := 0; i < 5; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("expected burst slot %d to pass", i)
		}
	}
	if rl.Allow(ip) {
		t.Fatal("expected 6th request within burst window to be denied")
	}
}

func TestRateLimiter_PerIP(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	for i := 0; i < 5; i++ {
		if !rl.Allow("1.1.1.1") {
			t.Fatalf("expected burst for IP 1.1.1.1")
		}
	}
	// Different IP gets its own bucket.
	if !rl.Allow("2.2.2.2") {
		t.Fatal("expected second IP to pass")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	// Rate=0 means no refill, so the burst of 2 is the only thing
	// available; subsequent requests are denied until enough time
	// passes. We freeze the clock so the test is deterministic.
	rl := NewRateLimiter(0, 2)
	frozen := time.Unix(0, 0)
	rl.now = func() time.Time { return frozen }

	var allowed, denied int
	h := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			denied++
		} else if rr.Code == http.StatusOK {
			allowed++
		} else {
			t.Fatalf("unexpected status %d", rr.Code)
		}
	}
	if allowed != 2 {
		t.Fatalf("expected 2 allowed, got %d", allowed)
	}
	if denied != 3 {
		t.Fatalf("expected 3 denied, got %d", denied)
	}
}


func TestRateLimiter_AllowByKey(t *testing.T) {
	rl := NewRateLimiter(10, 3)
	for i := 0; i < 3; i++ {
		allowed, _, _ := rl.AllowByKey("key-a")
		if !allowed {
			t.Fatalf("burst slot %d denied", i)
		}
	}
	allowed, remaining, retry := rl.AllowByKey("key-a")
	if allowed {
		t.Fatal("4th request should be denied")
	}
	if remaining != 0 {
		t.Fatalf("remaining: %d", remaining)
	}
	if retry <= 0 {
		t.Fatalf("retry should be positive: %v", retry)
	}
}

func TestRateLimiter_AllowByKey_IndependentKeys(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	if ok, _, _ := rl.AllowByKey("alpha"); !ok {
		t.Fatal("alpha should pass")
	}
	if ok, _, _ := rl.AllowByKey("beta"); !ok {
		t.Fatal("beta should pass (independent bucket)")
	}
}

func TestRateLimiter_AllowByKey_EmptyKey(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	// Empty key bypasses the limiter — used for unauthenticated paths.
	for i := 0; i < 100; i++ {
		if ok, _, _ := rl.AllowByKey(""); !ok {
			t.Fatalf("empty key should always pass (iteration %d)", i)
		}
	}
}

func TestRateLimiter_Stats(t *testing.T) {
	rl := NewRateLimiter(7, 14)
	rl.AllowByKey("a")
	rl.AllowByKey("b")
	s := rl.Stats()
	if s.Keys != 2 {
		t.Fatalf("keys=%d", s.Keys)
	}
	if s.Rate != 7 || s.Burst != 14 {
		t.Fatalf("got %+v", s)
	}
}
