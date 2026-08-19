package cipherio

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestCTRFromKey_BadKey(t *testing.T) {
	if _, err := CTRFromKey([]byte("short"), []byte("1234567890123456")); err == nil {
		t.Fatal("应报错")
	}
}

func TestCTRFromKey_BadNonce(t *testing.T) {
	k := make([]byte, 16)
	if _, err := CTRFromKey(k, []byte("x")); err == nil {
		t.Fatal("应报错")
	}
}

func TestRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := []byte("Hello, this is a secret message that should be encrypted.")
	var cipherBuf bytes.Buffer
	if err := EncryptCopy(&cipherBuf, bytes.NewReader(plaintext), key); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cipherBuf.Bytes(), plaintext) {
		t.Fatal("应被加密")
	}
	var out bytes.Buffer
	if err := DecryptCopy(&out, &cipherBuf, key); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), plaintext) {
		t.Fatal("roundtrip")
	}
}

func TestDecrypt_BadInput(t *testing.T) {
	k := make([]byte, 16)
	r := bytes.NewReader([]byte("abc"))
	if err := DecryptCopy(io.Discard, r, k); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal("应 EOF")
	}
}

func TestRandomNonce(t *testing.T) {
	n, err := RandomNonce(8)
	if err != nil || len(n) != 8 {
		t.Fatal("nonce")
	}
}
