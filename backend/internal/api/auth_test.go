package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIKeyAuth_Verify(t *testing.T) {
	a := NewAPIKeyAuth()
	a.Add("key-alpha", "alpha")
	a.Add("key-bravo", "bravo")

	if !a.Verify("key-alpha") {
		t.Fatal("expected key-alpha to verify")
	}
	if !a.Verify("key-bravo") {
		t.Fatal("expected key-bravo to verify")
	}
	if a.Verify("") {
		t.Fatal("empty key must not verify")
	}
	if a.Verify("nonexistent") {
		t.Fatal("unknown key must not verify")
	}
	if a.Verify("key-alpha-extra") {
		t.Fatal("longer key must not verify even with prefix match")
	}
}

func TestAPIKeyAuth_Middleware_Off(t *testing.T) {
	a := NewAPIKeyAuth()
	h := a.Middleware(AuthModeOff)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/services", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 in dev mode, got %d", rr.Code)
	}
}

func TestAPIKeyAuth_Middleware_RequiresKey(t *testing.T) {
	a := NewAPIKeyAuth()
	a.Add("secret", "test")
	h := a.Middleware(AuthModeAPIKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No key -> 401
	req := httptest.NewRequest("GET", "/api/services", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rr.Code)
	}
	if got := rr.Header().Get("WWW-Authenticate"); !strings.Contains(got, "Bearer") {
		t.Fatalf("expected WWW-Authenticate Bearer, got %q", got)
	}

	// Bearer header -> 200
	req = httptest.NewRequest("GET", "/api/services", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with bearer, got %d", rr.Code)
	}

	// X-API-Key header -> 200
	req = httptest.NewRequest("GET", "/api/services", nil)
	req.Header.Set("X-API-Key", "secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with X-API-Key, got %d", rr.Code)
	}

	// Wrong key -> 401
	req = httptest.NewRequest("GET", "/api/services", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong key, got %d", rr.Code)
	}
}

func TestAPIKeyAuth_PublicPathsBypass(t *testing.T) {
	a := NewAPIKeyAuth()
	a.Add("secret", "test")
	h := a.Middleware(AuthModeAPIKey, "/api/health", "/metrics")(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

	for _, path := range []string{"/api/health", "/metrics"} {
		req := httptest.NewRequest("GET", path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for public path %s, got %d", path, rr.Code)
		}
	}
}
