package jwt2

import (
	"errors"
	"testing"
)

func TestExtractBearer(t *testing.T) {
	tok, err := ExtractBearer("Bearer abc123")
	if err != nil || tok != "abc123" {
		t.Fatal("extract", tok, err)
	}
}

func TestExtractBearer_Missing(t *testing.T) {
	if _, err := ExtractBearer("Basic foo"); !errors.Is(err, ErrNoToken) {
		t.Fatal("miss")
	}
}

func TestExtractBearer_Empty(t *testing.T) {
	if _, err := ExtractBearer("Bearer "); !errors.Is(err, ErrNoToken) {
		t.Fatal("empty")
	}
}

func TestParseJWKS(t *testing.T) {
	js := `{"keys":[{"kty":"RSA","kid":"k1","n":"abc","e":"AQAB"},{"kty":"EC","kid":"k2","crv":"P-256"}]}`
	j, err := ParseJWKS([]byte(js))
	if err != nil {
		t.Fatal(err)
	}
	if len(j.Keys) != 2 {
		t.Fatal("keys", j)
	}
}

func TestFindKID(t *testing.T) {
	j := JWKS{Keys: []JWK{{Kid: "k1"}, {Kid: "k2"}}}
	k, ok := j.FindKID("k2")
	if !ok || k.Kid != "k2" {
		t.Fatal("find", k)
	}
	if _, ok := j.FindKID("k3"); ok {
		t.Fatal("miss")
	}
}

func TestParseJWKS_Bad(t *testing.T) {
	if _, err := ParseJWKS([]byte("not json")); err == nil {
		t.Fatal("bad")
	}
}
