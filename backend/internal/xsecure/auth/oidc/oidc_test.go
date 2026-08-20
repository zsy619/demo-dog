package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClaims_AllScopes(t *testing.T) {
	c := &Claims{Scope: "openid profile", Scopes: []string{"ingest:write"}, Groups: []string{"admins"}}
	set := map[string]struct{}{}
	for _, s := range c.AllScopes() {
		set[s] = struct{}{}
	}
	for _, want := range []string{"openid", "profile", "ingest:write", "group:admins"} {
		if _, ok := set[want]; !ok {
			t.Errorf("missing %s in %v", want, c.AllScopes())
		}
	}
}

func TestClaims_AllScopes_Empty(t *testing.T) {
	c := &Claims{}
	if len(c.AllScopes()) != 0 {
		t.Fatalf("expected empty, got %v", c.AllScopes())
	}
}

func TestAudienceContains(t *testing.T) {
	if !audienceContains([]string{"a", "b"}, "b") {
		t.Fatal("should contain b")
	}
	if audienceContains([]string{"a"}, "b") {
		t.Fatal("should not contain b")
	}
	if audienceContains(nil, "x") {
		t.Fatal("nil should not match")
	}
}

func TestRsaPubFromJWK(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01})
	k := JWK{Kty: "RSA", N: n, E: e}
	pk, err := rsaPubFromJWK(k)
	if err != nil {
		t.Fatal(err)
	}
	if pk.N.Cmp(key.N) != 0 {
		t.Fatal("N mismatch")
	}
}

func TestRsaPubFromJWK_BadBase64(t *testing.T) {
	k := JWK{Kty: "RSA", N: "!!!", E: "!!!"}
	if _, err := rsaPubFromJWK(k); err == nil {
		t.Fatal("expected error")
	}
}

func TestEcPubFromJWK_BadCurve(t *testing.T) {
	tk, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	k := JWK{Kty: "EC", Crv: "unknown",
		X: base64.RawURLEncoding.EncodeToString(tk.X.Bytes()),
		Y: base64.RawURLEncoding.EncodeToString(tk.Y.Bytes())}
	if _, err := ecPubFromJWK(k); err == nil {
		t.Fatal("expected error for unknown curve")
	}
}

func TestVerify_RSAToken(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwks := JWKS{Keys: []JWK{{
		Kty: "RSA", Kid: "k1", Alg: "RS256", Use: "sig",
		N: base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}}}
	provider, issuer, cleanup := newTestProvider(t, &jwks, rsaKey, nil)
	defer cleanup()
	token := signTestToken(t, &signTestOpts{
		alg:    "RS256",
		key:    rsaKey,
		header: map[string]any{"alg": "RS256", "kid": "k1", "typ": "JWT"},
		payload: map[string]any{
			"iss":   issuer,
			"sub":   "alice@example.com",
			"aud":   []string{"demo-dog"},
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"scope": "openid profile",
		},
	})
	t.Logf("issuer=%q cfg.IssuerURL=%q", issuer, provider.cfg.IssuerURL)
	claims, err := provider.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "alice@example.com" {
		t.Fatalf("sub: %s", claims.Subject)
	}
}

func TestVerify_BadAudience(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := JWKS{Keys: []JWK{{Kty: "RSA", Kid: "k1", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}}}
	provider, issuer, cleanup := newTestProvider(t, &jwks, rsaKey, nil)
	defer cleanup()
	token := signTestToken(t, &signTestOpts{
		alg:    "RS256",
		key:    rsaKey,
		header: map[string]any{"alg": "RS256", "kid": "k1"},
		payload: map[string]any{
			"iss": issuer,
			"sub": "alice",
			"aud": []string{"other-app"},
			"exp": time.Now().Add(time.Hour).Unix(),
		},
	})
	if _, err := provider.Verify(context.Background(), token); err == nil {
		t.Fatal("expected audience error")
	}
}

func TestVerify_Expired(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := JWKS{Keys: []JWK{{
		Kty: "RSA", Kid: "k1", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}}}
	provider, issuer, cleanup := newTestProvider(t, &jwks, rsaKey, nil)
	defer cleanup()
	token := signTestToken(t, &signTestOpts{
		alg:    "RS256",
		key:    rsaKey,
		header: map[string]any{"alg": "RS256", "kid": "k1"},
		payload: map[string]any{
			"iss": issuer,
			"sub": "alice",
			"aud": []string{"demo-dog"},
			"exp": time.Now().Add(-time.Hour).Unix(),
		},
	})
	if _, err := provider.Verify(context.Background(), token); err == nil {
		t.Fatal("expected expired error")
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	provider, _, cleanup := newTestProvider(t, &JWKS{}, nil, nil)
	defer cleanup()
	if _, err := provider.Verify(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty token")
	}
	if _, err := provider.Verify(context.Background(), "a.b"); err == nil {
		t.Fatal("expected error for 2-part token")
	}
}

func TestVerify_BadSignature(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := JWKS{Keys: []JWK{{
		Kty: "RSA", Kid: "k1", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}}}
	provider, issuer, cleanup := newTestProvider(t, &jwks, rsaKey, nil)
	defer cleanup()
	token := signTestToken(t, &signTestOpts{
		alg:    "RS256",
		key:    otherKey,
		header: map[string]any{"alg": "RS256", "kid": "k1"},
		payload: map[string]any{
			"iss": issuer,
			"sub": "alice",
			"aud": []string{"demo-dog"},
			"exp": time.Now().Add(time.Hour).Unix(),
		},
	})
	if _, err := provider.Verify(context.Background(), token); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestVerify_WrongIssuer(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := JWKS{Keys: []JWK{{
		Kty: "RSA", Kid: "k1", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}}}
	provider, _, cleanup := newTestProvider(t, &jwks, rsaKey, nil)
	defer cleanup()
	token := signTestToken(t, &signTestOpts{
		alg:    "RS256",
		key:    rsaKey,
		header: map[string]any{"alg": "RS256", "kid": "k1"},
		payload: map[string]any{
			"iss": "https://evil.example.com",
			"sub": "alice",
			"aud": []string{"demo-dog"},
			"exp": time.Now().Add(time.Hour).Unix(),
		},
	})
	if _, err := provider.Verify(context.Background(), token); err == nil {
		t.Fatal("expected issuer error")
	}
}

func TestVerify_MissingKid(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := JWKS{Keys: []JWK{{
		Kty: "RSA", Kid: "actual", Alg: "RS256",
		N: base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes()),
		E: base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}}}
	provider, issuer, cleanup := newTestProvider(t, &jwks, rsaKey, nil)
	defer cleanup()
	token := signTestToken(t, &signTestOpts{
		alg:    "RS256",
		key:    rsaKey,
		header: map[string]any{"alg": "RS256", "kid": "unknown"},
		payload: map[string]any{
			"iss": issuer,
			"sub": "alice",
			"aud": []string{"demo-dog"},
			"exp": time.Now().Add(time.Hour).Unix(),
		},
	})
	if _, err := provider.Verify(context.Background(), token); err == nil {
		t.Fatal("expected kid error")
	}
}

func TestNewProvider_RequiresConfig(t *testing.T) {
	if _, err := NewProvider(context.Background(), Config{}); err == nil {
		t.Fatal("expected error for empty config")
	}
	if _, err := NewProvider(context.Background(), Config{IssuerURL: "x"}); err == nil {
		t.Fatal("expected error for missing clientID")
	}
}

type signTestOpts struct {
	alg     string
	key     interface{}
	header  map[string]any
	payload map[string]any
}

func signTestToken(t *testing.T, o *signTestOpts) string {
	t.Helper()
	enc := base64.RawURLEncoding.EncodeToString
	hBytes, _ := json.Marshal(o.header)
	pBytes, _ := json.Marshal(o.payload)
	signingInput := enc(hBytes) + "." + enc(pBytes)
	var sig []byte
	var err error
	switch o.alg {
	case "RS256":
		rsaKey := o.key.(*rsa.PrivateKey)
		sig, err = rsa.SignPKCS1v15(rand.Reader, rsaKey, 0, sumSHA([]byte(signingInput)))
	}
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + enc(sig)
}

func sumSHA(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
}

func newTestProvider(t *testing.T, jwks *JWKS, rsaKey *rsa.PrivateKey, ecKey *ecdsa.PrivateKey) (*OIDCProvider, string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DiscoveryDoc{
			Issuer:  ts.URL,
			JWKSURI: ts.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	})
	provider := &OIDCProvider{
		cfg:        Config{IssuerURL: ts.URL, ClientID: "demo-dog", JWKSRefresh: time.Minute},
		httpClient: ts.Client(),
	}
	provider.discovery = &DiscoveryDoc{Issuer: ts.URL, JWKSURI: ts.URL + "/jwks"}
	provider.jwks = jwks
	return provider, ts.URL, ts.Close
}
