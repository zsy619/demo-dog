package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeMTLS struct {
	allow map[string]string
}

func (f *fakeMTLS) VerifyPeer(sub string) (string, bool) {
	t, ok := f.allow[sub]
	return t, ok
}

type fakeOIDC struct {
	fail bool
}

func (f *fakeOIDC) VerifyToken(ctx context.Context, raw string) (string, string, []string, error) {
	if f.fail {
		return "", "", nil, errors.New("bad token")
	}
	return "alice", "acme", []string{"rules:read"}, nil
}

func newReq(auth string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	return r
}

func TestPrincipal_HasScope(t *testing.T) {
	p := Principal{}
	if !p.HasScope("any") {
		t.Fatal("empty scopes = permissive")
	}
	p.Scopes = []string{"a"}
	if !p.HasScope("a") {
		t.Fatal("should match")
	}
	if p.HasScope("b") {
		t.Fatal("should not match")
	}
}

func TestPrincipal_IsAdmin(t *testing.T) {
	if !(Principal{Identity: "admin"}).IsAdmin() {
		t.Fatal("identity admin")
	}
	if !(Principal{Scopes: []string{"admin"}}).IsAdmin() {
		t.Fatal("scope admin")
	}
	if (Principal{Identity: "user"}).IsAdmin() {
		t.Fatal("non-admin")
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := WithPrincipal(context.Background(), Principal{Subject: "x"})
	p, ok := PrincipalFromContext(ctx)
	if !ok || p.Subject != "x" {
		t.Fatal("roundtrip")
	}
}

func TestDecodeAuthorization(t *testing.T) {
	k, r, err := DecodeAuthorization("Bearer abc")
	if err != nil || k != "Bearer" || r != "abc" {
		t.Fatal("bad split")
	}
	if _, _, err := DecodeAuthorization(""); err == nil {
		t.Fatal("empty should error")
	}
}

func TestComposeBearer(t *testing.T) {
	if ComposeBearer("x") != "Bearer x" {
		t.Fatal("compose")
	}
}

func TestAuthenticate_Bearer(t *testing.T) {
	pm := NewPrincipalMap()
	pm.Register("tok", Principal{Subject: "alice", Tenant: "acme", Identity: "admin", Scopes: []string{"rules:read"}})
	mw := pm.AsMiddleware()
	p, err := mw.Authenticate(newReq("Bearer tok"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "alice" || p.Tenant != "acme" || p.Identity != "admin" {
		t.Fatalf("%+v", p)
	}
}

func TestAuthenticate_Bearer_Missing(t *testing.T) {
	mw := (&PrincipalMap{}).AsMiddleware()
	if _, err := mw.Authenticate(newReq("Bearer nope")); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthenticate_NoAuth(t *testing.T) {
	mw := (&PrincipalMap{}).AsMiddleware()
	if _, err := mw.Authenticate(newReq("")); !errors.Is(err, ErrNoAuth) {
		t.Fatal("expected ErrNoAuth")
	}
}

func TestAuthenticate_ExpiredBearer(t *testing.T) {
	mw := &Middleware{
		Bearer: map[string]bearerEntry{
			"x": BearerEntry{Valid: func(time.Time) bool { return false }},
		},
	}
	if _, err := mw.Authenticate(newReq("Bearer x")); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("expected ErrUnauthorized")
	}
}

func TestAuthenticate_MTLSWinsOverBearer(t *testing.T) {
	pm := NewPrincipalMap()
	pm.Register("tok", Principal{Subject: "bearer"})
	mw := pm.AsMiddleware()
	mw.MTLS = &fakeMTLS{allow: map[string]string{"client-cn": "acme"}}
	mw.PeerCert = func(r *http.Request) string { return "client-cn" }
	p, _ := mw.Authenticate(newReq("Bearer tok"))
	if p.Method != "mtls" {
		t.Fatalf("mTLS should win: %+v", p)
	}
}

func TestAuthenticate_OIDC(t *testing.T) {
	mw := &Middleware{OIDC: &fakeOIDC{}}
	p, err := mw.Authenticate(newReq("Bearer-OIDC eyJ..."))
	if err != nil {
		t.Fatal(err)
	}
	if p.Method != "oidc" || p.Subject != "alice" {
		t.Fatalf("%+v", p)
	}
}

func TestAuthenticate_OIDC_Failure(t *testing.T) {
	mw := &Middleware{OIDC: &fakeOIDC{fail: true}}
	if _, err := mw.Authenticate(newReq("Bearer-OIDC bad")); err == nil {
		t.Fatal("expected error")
	}
}

func TestRequireAny_401(t *testing.T) {
	mw := (&PrincipalMap{}).AsMiddleware()
	h := mw.RequireAny(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, newReq(""))
	if rr.Code != 401 {
		t.Fatalf("status: %d", rr.Code)
	}
}

func TestRequireAny_Passes(t *testing.T) {
	pm := NewPrincipalMap()
	pm.Register("tok", Principal{Subject: "x"})
	mw := pm.AsMiddleware()
	var seen Principal
	h := mw.RequireAny(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = PrincipalFromContext(r.Context())
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, newReq("Bearer tok"))
	if rr.Code != 200 {
		t.Fatalf("status: %d", rr.Code)
	}
	if seen.Subject != "x" {
		t.Fatalf("seen: %+v", seen)
	}
}

func TestRequireRole(t *testing.T) {
	pm := NewPrincipalMap()
	pm.Register("tok", Principal{Identity: "user"})
	mw := pm.AsMiddleware()
	h := mw.RequireRole("admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, newReq("Bearer tok"))
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestRequireScope(t *testing.T) {
	pm := NewPrincipalMap()
	pm.Register("tok", Principal{Scopes: []string{"metrics:read"}})
	mw := pm.AsMiddleware()
	h := mw.RequireScope("rules:read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, newReq("Bearer tok"))
	if rr.Code != 403 {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestHasScope(t *testing.T) {
	r := newReq("")
	if HasScope(r, "x") {
		t.Fatal("no principal should be false")
	}
}

func TestHashToken(t *testing.T) {
	a := HashToken("hello")
	b := HashToken("hello")
	if a != b {
		t.Fatal("hash not deterministic")
	}
	if len(a) != 64 {
		t.Fatal("sha256 hex is 64 chars")
	}
}

func TestCompareTokens(t *testing.T) {
	if !CompareTokens("x", "x") {
		t.Fatal("equal")
	}
	if CompareTokens("x", "y") {
		t.Fatal("not equal")
	}
}
