package cap

import (
	"errors"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	secret := []byte("secret")
	tk, err := Issued(secret, "alice", "users/*", []string{"read", "write"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	tk1, err := Verify(secret, tk)
	if err != nil {
		t.Fatal(err)
	}
	if tk1.Subject != "alice" {
		t.Fatal("sub")
	}
}

func TestBadSignature(t *testing.T) {
	tk, _ := Issued([]byte("a"), "x", "", []string{"r"}, time.Minute)
	if _, err := Verify([]byte("b"), tk); !errors.Is(err, ErrInvalid) {
		t.Fatal("应 ErrInvalid")
	}
}

func TestExpired(t *testing.T) {
	tk, _ := Issued([]byte("s"), "x", "", []string{"r"}, -time.Minute)
	if _, err := Verify([]byte("s"), tk); !errors.Is(err, ErrExpired) {
		t.Fatal("应 ErrExpired")
	}
}

func TestAuthorize_OK(t *testing.T) {
	secret := []byte("s")
	tk, _ := Issued(secret, "a", "users", []string{"read"}, time.Minute)
	if err := Authorize(secret, tk, "read", "users"); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorize_ResourceMismatch(t *testing.T) {
	secret := []byte("s")
	tk, _ := Issued(secret, "a", "orders", []string{"read"}, time.Minute)
	if err := Authorize(secret, tk, "read", "users"); err == nil {
		t.Fatal("应拒绝")
	}
}

func TestAuthorize_ScopeMismatch(t *testing.T) {
	secret := []byte("s")
	tk, _ := Issued(secret, "a", "", []string{"read"}, time.Minute)
	if err := Authorize(secret, tk, "write", ""); err == nil {
		t.Fatal("应拒绝")
	}
}

func TestHasScope_Wildcard(t *testing.T) {
	tk := &Token{Scopes: []string{"*"}}
	if !tk.HasScope("any") {
		t.Fatal("wildcard")
	}
}

func TestHasResource_Empty(t *testing.T) {
	tk := &Token{}
	if !tk.HasResource("x") {
		t.Fatal("空 resource 应通过")
	}
}

func TestSecret(t *testing.T) {
	s, _ := Secret()
	if len(s) != 32 {
		t.Fatal("secret")
	}
}

func TestCache(t *testing.T) {
	c := NewCache()
	tk := &Token{Subject: "x"}
	c.Put("raw", tk)
	if got, ok := c.Get("raw"); !ok || got.Subject != "x" {
		t.Fatal("cache")
	}
	c.Clear()
	if _, ok := c.Get("raw"); ok {
		t.Fatal("clear")
	}
}
