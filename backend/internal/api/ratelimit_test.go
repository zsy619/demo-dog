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
