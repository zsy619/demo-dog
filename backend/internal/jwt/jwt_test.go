package jwt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newV() *Verifier {
	return New(time.Second).WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func newKey(kid string) *Key {
	return &Key{KID: kid, Secret: []byte("s-" + kid), Alg: HS256}
}

func TestVerify_Happy(t *testing.T) {
	v := newV()
	v.Add(newKey("k1"))
	tok, err := Sign(map[string]any{"sub": "alice"}, newKey("k1"))
	if err != nil {
		t.Fatal(err)
	}
	claims, kid, err := v.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != "alice" || kid != "k1" {
		t.Fatal(claims)
	}
}

func TestVerify_BadKid(t *testing.T) {
	v := newV()
	v.Add(newKey("k1"))
	tok, _ := Sign(map[string]any{}, newKey("k2"))
	if _, _, err := v.Verify(tok); !errors.Is(err, ErrNoKey) {
		t.Fatal(err)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	v := newV()
	v.Add(newKey("k1"))
	tok, _ := Sign(map[string]any{}, newKey("k1"))
	if _, _, err := v.Verify(tok + "x"); !errors.Is(err, ErrBadToken) {
		t.Fatal(err)
	}
}

func TestVerify_RotatingKey(t *testing.T) {
	v := newV()
	v.Add(newKey("k1"))
	tok, _ := Sign(map[string]any{}, newKey("k1"))
	v.Add(newKey("k2"))
	if _, _, err := v.Verify(tok); err != nil {
		t.Fatal("old key should still verify")
	}
	v.Remove("k1")
	if _, _, err := v.Verify(tok); !errors.Is(err, ErrNoKey) {
		t.Fatal("old key should fail after Remove")
	}
}

func TestVerify_Expired(t *testing.T) {
	now := time.Unix(1700000000, 0)
	v := New(time.Second).WithTime(func() time.Time { return now })
	v.Add(newKey("k1"))
	tok, _ := Sign(map[string]any{"exp": float64(now.Unix())}, newKey("k1"))
	now = now.Add(2 * time.Second)
	if _, _, err := v.Verify(tok); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
}

func TestVerify_BadFormat(t *testing.T) {
	v := newV()
	if _, _, err := v.Verify("not-a-token"); !errors.Is(err, ErrBadToken) {
		t.Fatal(err)
	}
}

func TestVerify_Nbf(t *testing.T) {
	now := time.Unix(1700000000, 0)
	v := New(time.Second).WithTime(func() time.Time { return now })
	v.Add(newKey("k1"))
	tok, _ := Sign(map[string]any{"nbf": float64(now.Unix() + 30)}, newKey("k1"))
	if _, _, err := v.Verify(tok); !errors.Is(err, ErrBadToken) {
		t.Fatal(err)
	}
}

func TestMiddleware_Allows(t *testing.T) {
	v := newV()
	v.Add(newKey("k1"))
	tok, _ := Sign(map[string]any{"sub": "alice"}, newKey("k1"))
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _, ok := FromContext(r.Context())
		if !ok || c["sub"] != "alice" {
			t.Fatal("claims not propagated")
		}
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(rr, r)
	if rr.Code != 200 {
		t.Fatal("status")
	}
}

func TestMiddleware_Blocks(t *testing.T) {
	v := newV()
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatal("status")
	}
}

func TestFromContext_Missing(t *testing.T) {
	if _, _, ok := FromContext(noopCtx{}); ok {
		t.Fatal("missing")
	}
}

type noopCtx struct{}

func (noopCtx) Value(any) any { return nil }
func (noopCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (noopCtx) Done() <-chan struct{} { return nil }
func (noopCtx) Err() error { return nil }

var _ context.Context = noopCtx{}
