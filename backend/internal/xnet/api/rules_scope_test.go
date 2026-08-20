package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zsy619/demo-dog/backend/internal/xdata/ingest"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
	"github.com/zsy619/demo-dog/backend/internal/xflow/stream"
)

func newRulesScopeServer(t *testing.T) *Server {
	t.Helper()
	d := store.New(store.DefaultConfig())
	hub := stream.NewHub()
	in := ingest.New(d, 4)
	s := New(d, in, hub)
	return s
}

func TestRules_Scope_Forbidden(t *testing.T) {
	s := newRulesScopeServer(t)
	// Key with non-empty scopes that don't include rules:read.
	s.Auth().AddWithScopes("k1", "reader", "acme", RoleReader,
		[]string{"metrics:read", "logs:read"})
	s.SetAuthMode(AuthModeAPIKey)
	req := httptest.NewRequest("GET", "/api/v1/rules", nil)
	req.Header.Set("X-API-Key", "k1")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without rules:read, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRules_Scope_Allowed(t *testing.T) {
	s := newRulesScopeServer(t)
	s.Auth().AddWithScopes("k2", "reader", "acme", RoleReader,
		[]string{"rules:read"})
	s.SetAuthMode(AuthModeAPIKey)
	req := httptest.NewRequest("GET", "/api/v1/rules", nil)
	req.Header.Set("X-API-Key", "k2")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with rules:read, got %d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "success" {
		t.Fatalf("status: %v", body["status"])
	}
}

func TestRules_Scope_MultipleKeys(t *testing.T) {
	s := newRulesScopeServer(t)
	s.Auth().AddWithScopes("admin", "admin", "acme", RoleAdmin,
		[]string{"rules:read", "rules:write", "ingest:write"})
	s.Auth().AddWithScopes("viewer", "viewer", "acme", RoleReader,
		[]string{"metrics:read"})
	s.SetAuthMode(AuthModeAPIKey)

	for _, c := range []struct {
		key  string
		code int
	}{
		{"admin", http.StatusOK},
		{"viewer", http.StatusForbidden},
	} {
		req := httptest.NewRequest("GET", "/api/v1/rules", nil)
		req.Header.Set("X-API-Key", c.key)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != c.code {
			t.Errorf("key=%s expected %d, got %d body=%s",
				c.key, c.code, w.Code, w.Body.String())
		}
	}
}

func TestRules_Scope_AuthOff(t *testing.T) {
	// When auth is off, scope check is skipped (dev mode).
	s := newRulesScopeServer(t)
	s.SetAuthMode(AuthModeOff)
	req := httptest.NewRequest("GET", "/api/v1/rules", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("auth off expected 200, got %d", w.Code)
	}
}

func TestRules_Scope_ForbiddenMessage(t *testing.T) {
	s := newRulesScopeServer(t)
	s.Auth().AddWithScopes("k1", "reader", "acme", RoleReader,
		[]string{"other:scope"})
	s.SetAuthMode(AuthModeAPIKey)
	req := httptest.NewRequest("GET", "/api/v1/rules", nil)
	req.Header.Set("X-API-Key", "k1")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if !contains(w.Body.String(), "rules:read") {
		t.Fatalf("error body should mention rules:read: %s", w.Body.String())
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
