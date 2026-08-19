package jwt

import (
	"errors"
	"testing"
	"time"
)

func TestSignVerify(t *testing.T) {
	secret := []byte("s")
	tk, err := Sign(HS256, secret, Claims{"sub": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := Verify(HS256, secret, tk)
	if err != nil {
		t.Fatal(err)
	}
	if c["sub"] != "alice" {
		t.Fatal("sub")
	}
}

func TestBadSignature(t *testing.T) {
	tk, _ := Sign(HS256, []byte("a"), Claims{"sub": "x"})
	if _, err := Verify(HS256, []byte("b"), tk); !errors.Is(err, ErrBadSignature) {
		t.Fatal("应 ErrBadSignature")
	}
}

func TestBadFormat(t *testing.T) {
	if _, err := Verify(HS256, []byte("x"), "x.y"); !errors.Is(err, ErrBadFormat) {
		t.Fatal("应 ErrBadFormat")
	}
}

func TestExpired(t *testing.T) {
	tk, _ := Issue([]byte("s"), "x", -time.Minute, nil)
	if _, err := Verify(HS256, []byte("s"), tk); !errors.Is(err, ErrExpired) {
		t.Fatal("应 ErrExpired")
	}
}

func TestIssue(t *testing.T) {
	tk, _ := Issue([]byte("s"), "alice", time.Minute, Claims{"role": "admin"})
	c, _ := Verify(HS256, []byte("s"), tk)
	if c["sub"] != "alice" || c["role"] != "admin" {
		t.Fatal("issue")
	}
}
