package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// W1.6: 限流按 scope 隔离。

func TestRateLimiter_AllowByKeyScoped_IndependentBuckets(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 token/s, burst 1
	defer func() {}()
	if ok, _, _ := rl.AllowByKeyScoped("key-1", "ingest"); !ok {
		t.Fatal("ingest #1 must allow")
	}
	if ok, _, _ := rl.AllowByKeyScoped("key-1", "ingest"); ok {
		t.Fatal("ingest #2 must reject")
	}
	// query scope 仍可放行
	if ok, _, _ := rl.AllowByKeyScoped("key-1", "query"); !ok {
		t.Fatal("query scope must be independent")
	}
	// billing 同理
	if ok, _, _ := rl.AllowByKeyScoped("key-1", "billing"); !ok {
		t.Fatal("billing scope must be independent")
	}
}

func TestRateLimiter_AllowByKeyScoped_EmptyScopeFallsBack(t *testing.T) {
	rl := NewRateLimiter(1, 2)
	if ok, _, _ := rl.AllowByKeyScoped("key-1", ""); !ok {
		t.Fatal("empty scope must allow #1")
	}
	if ok, _, _ := rl.AllowByKeyScoped("key-1", ""); !ok {
		t.Fatal("empty scope must allow #2 (burst=2)")
	}
	if ok, _, _ := rl.AllowByKeyScoped("key-1", ""); ok {
		t.Fatal("empty scope must reject #3")
	}
}

func TestRateLimiter_AllowByKeyScoped_EmptyKeyAllowed(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	if ok, _, _ := rl.AllowByKeyScoped("", "anything"); !ok {
		t.Fatal("empty key must always be allowed")
	}
}

func TestRateLimiter_ScopedKeyedMiddleware_RetryAfterOn429(t *testing.T) {
	rl := NewRateLimiter(0.1, 1) // 0.1 token/s, burst 1
	defer func() {}()
	// 等同于 KeyedMiddleware 但 scope 来自 header。
	var reqScope string
	h := rl.ScopedKeyedMiddleware(
		func(r *http.Request) string { return "client-A" },
		func(r *http.Request) string { return reqScope },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 第一次 ingest 通过
	reqScope = "ingest"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("first ingest must pass, got %d", rr.Code)
	}
	if rr.Header().Get("X-RateLimit-Scope") != "ingest" {
		t.Errorf("missing scope header: %s", rr.Header().Get("X-RateLimit-Scope"))
	}

	// 第二次 ingest 应 429 + Retry-After
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second ingest must 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}
	if rr.Header().Get("X-RateLimit-Scope") != "ingest" {
		t.Errorf("scope header missing on 429: %q",
			rr.Header().Get("X-RateLimit-Scope"))
	}

	// 同一 client 但不同 scope 必须放行
	reqScope = "query"
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("different scope must be independent, got %d", rr.Code)
	}
}

func TestRateLimiter_ScopedKeyedMiddleware_EmptyKeyBypass(t *testing.T) {
	rl := NewRateLimiter(0, 0) // 关停 + 1 token 兜底
	h := rl.ScopedKeyedMiddleware(
		func(r *http.Request) string { return "" },
		func(r *http.Request) string { return "" },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("empty key must bypass, got %d", rr.Code)
	}
}

func TestRateLimiter_AllowByKeyScoped_RetryAfterSeconds(t *testing.T) {
	rl := NewRateLimiter(0.5, 1) // 0.5 token/s = 2s refill
	if ok, _, _ := rl.AllowByKeyScoped("k", "ingest"); !ok {
		t.Fatal("first must allow")
	}
	ok, _, retry := rl.AllowByKeyScoped("k", "ingest")
	if ok {
		t.Fatal("second must reject")
	}
	if retry < time.Second || retry > 3*time.Second {
		t.Errorf("retry should be ~2s, got %v", retry)
	}
}
