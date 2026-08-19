package sealedx

import "testing"

func TestRoundTrip(t *testing.T) {
	k := []byte("0123456789abcdef")
	pt := []byte("secret data")
	ct, err := Seal(k, pt)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Open(k, ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(pt) {
		t.Fatal("rt")
	}
}

func TestWrongKey(t *testing.T) {
	pt, _ := Seal([]byte("0123456789abcdef"), []byte("hi"))
	if _, err := Open([]byte("zyxwvutsrqponmlk"), pt); err == nil {
		t.Fatal("wrong")
	}
}

func TestBadKey(t *testing.T) {
	if _, err := Seal([]byte("short"), []byte("x")); err == nil {
		t.Fatal("bad key")
	}
}

func TestShortBlob(t *testing.T) {
	if _, err := Open([]byte("0123456789abcdef"), []byte("abc")); err == nil {
		t.Fatal("short")
	}
}
