package aesx

import "testing"

func TestRoundTrip(t *testing.T) {
	k, _ := RandomKey(128)
	ct, err := Encrypt(k, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := Decrypt(k, ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != "hi" {
		t.Fatal("rt", pt)
	}
}

func TestBadKey(t *testing.T) {
	if _, err := Encrypt([]byte("short"), []byte("x")); err == nil {
		t.Fatal("bad")
	}
}

func TestDecryptShort(t *testing.T) {
	if _, err := Decrypt(make([]byte, 16), []byte("abc")); err == nil {
		t.Fatal("short")
	}
}

func TestRandomKey(t *testing.T) {
	k, err := RandomKey(128)
	if err != nil || len(k) != 16 {
		t.Fatal("r128", err, len(k))
	}
	k, _ = RandomKey(256)
	if len(k) != 32 {
		t.Fatal("r256", len(k))
	}
}

func TestRandomKeyBad(t *testing.T) {
	if _, err := RandomKey(100); err == nil {
		t.Fatal("bad bits")
	}
}

func TestTampered(t *testing.T) {
	k, _ := RandomKey(128)
	ct, _ := Encrypt(k, []byte("hello"))
	ct[len(ct)-1] ^= 0xff
	if _, err := Decrypt(k, ct); err == nil {
		t.Fatal("篡改应失败")
	}
}
