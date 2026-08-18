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

func TestParseRole(t *testing.T) {
	cases := map[string]Role{
		"admin":    RoleAdmin,
		"writer":   RoleWriter,
		"reader":   RoleReader,
		"RW":       RoleWriter,
		"":         RoleReader,
		"unknown":  RoleReader,
	}
	for s, want := range cases {
		if got := ParseRole(s); got != want {
			t.Errorf("ParseRole(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestAPIKeyAuth_AddFromSpec(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddFromSpec("k1:admin:alice")
	a.AddFromSpec("k2:writer:checkout")
	a.AddFromSpec("k3:reader:grafana")
	a.AddFromSpec("k4") // default writer

	if r, _ := a.RoleOf("k1"); r != RoleAdmin {
		t.Errorf("k1 role = %v, want admin", r)
	}
	if r, _ := a.RoleOf("k2"); r != RoleWriter {
		t.Errorf("k2 role = %v, want writer", r)
	}
	if r, _ := a.RoleOf("k3"); r != RoleReader {
		t.Errorf("k3 role = %v, want reader", r)
	}
	if r, _ := a.RoleOf("k4"); r != RoleWriter {
		t.Errorf("k4 default role = %v, want writer", r)
	}
	if got := a.LabelOf("k1"); got != "alice" {
		t.Errorf("k1 label = %q, want alice", got)
	}
}

func TestAPIKeyAuth_RequireRole(t *testing.T) {
	a := NewAPIKeyAuth()
	a.Add("r", "reader", RoleReader)
	a.Add("w", "writer", RoleWriter)
	a.Add("x", "admin", RoleAdmin)

	base := a.Middleware(AuthModeAPIKey)

	for _, tc := range []struct {
		name     string
		key      string
		min      Role
		wantCode int
	}{
		{"reader cannot write", "r", RoleWriter, http.StatusForbidden},
		{"writer can write", "w", RoleWriter, http.StatusOK},
		{"admin can write", "x", RoleWriter, http.StatusOK},
		{"admin can admin", "x", RoleAdmin, http.StatusOK},
		{"reader can read", "r", RoleReader, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := RequireRole(tc.min, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			h := base(inner)
			req := httptest.NewRequest("GET", "/x", nil)
			req.Header.Set("Authorization", "Bearer "+tc.key)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.wantCode {
				t.Fatalf("got %d, want %d", rr.Code, tc.wantCode)
			}
		})
	}
}

func TestAPIKeyAuth_List(t *testing.T) {
	a := NewAPIKeyAuth()
	a.AddFromSpec("k1:admin:alice")
	a.AddFromSpec("k2:writer:checkout")

	entries := a.List()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	roles := map[string]Role{}
	for _, e := range entries {
		roles[e.Key] = e.Role
	}
	if roles["k1"] != RoleAdmin || roles["k2"] != RoleWriter {
		t.Errorf("role map wrong: %+v", roles)
	}
}
