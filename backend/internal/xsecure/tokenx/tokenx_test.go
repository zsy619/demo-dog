package tokenx

import (
	"errors"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	k := []byte("secret")
	tk, err := Sign(Payload{Subject: "alice"}, k)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Verify(tk, k, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "alice" {
		t.Fatal("sub", p.Subject)
	}
}

func TestBadSig(t *testing.T) {
	tk, _ := Sign(Payload{Subject: "x"}, []byte("a"))
	bad := tk + "x"
	if _, err := Verify(bad, []byte("a"), 0); !errors.Is(err, ErrBadSig) {
		t.Fatal("sig")
	}
}

func TestBadFormat(t *testing.T) {
	if _, err := Verify("onlyone", []byte("a"), 0); !errors.Is(err, ErrBadFormat) {
		t.Fatal("fmt")
	}
}

func TestExpired(t *testing.T) {
	tk, _ := Sign(Payload{ExpiresAt: 100}, []byte("a"))
	if _, err := Verify(tk, []byte("a"), 200); err == nil {
		t.Fatal("exp")
	}
}

func TestWrongSecret(t *testing.T) {
	tk, _ := Sign(Payload{}, []byte("a"))
	if _, err := Verify(tk, []byte("b"), 0); !errors.Is(err, ErrBadSig) {
		t.Fatal("wrong")
	}
}

func TestRandomSecret(t *testing.T) {
	s, err := RandomSecret()
	if err != nil || len(s) != 32 {
		t.Fatal("rs")
	}
}
