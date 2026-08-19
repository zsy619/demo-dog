package xorx

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	c := New([]byte("k"))
	pt := []byte("hello")
	ct := c.Encrypt(pt)
	if bytes.Equal(ct, pt) {
		t.Fatal("应加密")
	}
	out := c.Decrypt(ct)
	if !bytes.Equal(out, pt) {
		t.Fatal("rt")
	}
}

func TestBytes(t *testing.T) {
	pt := []byte("data")
	ct := Bytes([]byte("key"), pt)
	if bytes.Equal(ct, pt) {
		t.Fatal("bytes")
	}
}

func TestKeyIsolation(t *testing.T) {
	c := New([]byte("a"))
	c.Encrypt([]byte("data"))
	if c.key[0] != 'a' {
		t.Fatal("key 应不变")
	}
}


