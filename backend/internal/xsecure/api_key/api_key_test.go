package api_key

import (
	"errors"
	"testing"
)

func TestIssueVerify(t *testing.T) {
	m := New("secret")
	tk := m.Issue("alice")
	id, err := m.Verify(tk)
	if err != nil || id != "alice" {
		t.Fatal("issue")
	}
}

func TestVerify_BadPrefix(t *testing.T) {
	m := New("s")
	if _, err := m.Verify("bad.key"); !errors.Is(err, ErrBadFormat) {
		t.Fatal("prefix")
	}
}

func TestVerify_BadFormat(t *testing.T) {
	m := New("s")
	if _, err := m.Verify("ak_x"); !errors.Is(err, ErrBadFormat) {
		t.Fatal("format")
	}
}

func TestVerify_BadSig(t *testing.T) {
	m := New("s")
	tk := m.Issue("x")
	bad := tk[:len(tk)-1] + "X"
	if _, err := m.Verify(bad); !errors.Is(err, ErrBadSignature) {
		t.Fatal("sig")
	}
}

func TestHash(t *testing.T) {
	a := Hash("ak_x.y")
	b := Hash("ak_x.y")
	if a != b {
		t.Fatal("hash stable")
	}
	if a == "" {
		t.Fatal("empty")
	}
}

func TestStripPrefix(t *testing.T) {
	if StripPrefix("ak_x.y") != "x.y" {
		t.Fatal("strip")
	}
}
