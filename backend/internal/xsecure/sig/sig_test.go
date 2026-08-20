package sig

import (
	"crypto/ed25519"
	"errors"
	"testing"
)

func TestGenerate(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(kp.Public) != ed25519.PublicKeySize {
		t.Fatal("public size")
	}
	if len(kp.Private) != ed25519.PrivateKeySize {
		t.Fatal("private size")
	}
}

func TestSignVerify(t *testing.T) {
	kp, _ := Generate()
	msg := []byte("hello")
	s := kp.Sign(msg)
	if !kp.Verify(msg, s) {
		t.Fatal("self verify")
	}
}

func TestVerify_Tamper(t *testing.T) {
	kp, _ := Generate()
	msg := []byte("hello")
	s := kp.Sign(msg)
	if kp.Verify([]byte("hellp"), s) {
		t.Fatal("should not verify")
	}
}

func TestVerify_WrongKey(t *testing.T) {
	kp1, _ := Generate()
	kp2, _ := Generate()
	msg := []byte("hello")
	s := kp1.Sign(msg)
	if kp2.Verify(msg, s) {
		t.Fatal("wrong key should not verify")
	}
}

func TestVerifier_Register(t *testing.T) {
	v := NewVerifier()
	kp, _ := Generate()
	v.Add("k1", kp.Public)
	if err := v.VerifyMessage("k1", []byte("x"), kp.Sign([]byte("x"))); err != nil {
		t.Fatal(err)
	}
}

func TestVerifier_Unknown(t *testing.T) {
	v := NewVerifier()
	if err := v.VerifyMessage("missing", []byte("x"), []byte{}); !errors.Is(err, ErrUnknownKey) {
		t.Fatal(err)
	}
}

func TestVerifier_BadSignature(t *testing.T) {
	v := NewVerifier()
	kp, _ := Generate()
	v.Add("k1", kp.Public)
	if err := v.VerifyMessage("k1", []byte("x"), []byte("bad")); !errors.Is(err, ErrBadSignature) {
		t.Fatal(err)
	}
}

func TestVerifier_Remove(t *testing.T) {
	v := NewVerifier()
	kp, _ := Generate()
	v.Add("k1", kp.Public)
	v.Remove("k1")
	if err := v.VerifyMessage("k1", []byte("x"), []byte{}); !errors.Is(err, ErrUnknownKey) {
		t.Fatal(err)
	}
}

func TestVerifier_IDs(t *testing.T) {
	v := NewVerifier()
	kp, _ := Generate()
	v.Add("a", kp.Public)
	v.Add("b", kp.Public)
	ids := v.IDs()
	if len(ids) != 2 {
		t.Fatal("ids")
	}
}
