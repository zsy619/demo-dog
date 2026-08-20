package cipher

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestSealOpenRoundtrip(t *testing.T) {
	key, _ := RandomKey(256)
	defer Zero(key)
	ct, err := SealAESGCM(key, []byte("hello world"), nil)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := OpenAESGCM(key, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer Zero(pt)
	if !bytes.Equal(pt, []byte("hello world")) {
		t.Fatal("plaintext mismatch")
	}
}

func TestSealWithAAD(t *testing.T) {
	key, _ := RandomKey(256)
	defer Zero(key)
	ct, err := SealAESGCM(key, []byte("x"), []byte("context-A"))
	if err != nil {
		t.Fatal(err)
	}
	// 错误 AAD 应失败
	if _, err := OpenAESGCM(key, ct, []byte("context-B")); err == nil {
		t.Fatal("AAD mismatch 应报错")
	}
	// 正确 AAD 应成功
	if _, err := OpenAESGCM(key, ct, []byte("context-A")); err != nil {
		t.Fatal("err:", err)
	}
}

func TestKeyLength(t *testing.T) {
	bad := make([]byte, 8)
	if _, err := SealAESGCM(bad, []byte("x"), nil); err != ErrKeyLength {
		t.Fatal("应 ErrKeyLength")
	}
	if _, err := OpenAESGCM(bad, []byte("short"), nil); err != ErrKeyLength {
		t.Fatal("应 ErrKeyLength")
	}
}

func TestShortCiphertext(t *testing.T) {
	key, _ := RandomKey(256)
	defer Zero(key)
	if _, err := OpenAESGCM(key, []byte("short"), nil); err != ErrShortMessage {
		t.Fatal("应 ErrShortMessage")
	}
}

func TestRandomKey(t *testing.T) {
	for _, bits := range []int{128, 192, 256} {
		k, err := RandomKey(bits)
		if err != nil {
			t.Fatal(err)
		}
		if len(k) != bits/8 {
			t.Fatal(bits, len(k))
		}
		Zero(k)
	}
	if _, err := RandomKey(100); err != ErrInvalidBits {
		t.Fatal("应 ErrInvalidBits")
	}
}

func TestDeriveKey(t *testing.T) {
	k1 := DeriveKey("secret")
	k2 := DeriveKey("secret")
	if !bytes.Equal(k1, k2) {
		t.Fatal("DeriveKey 应可重现")
	}
	if len(k1) != 32 {
		t.Fatal("len")
	}
}

func TestZero(t *testing.T) {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i + 1)
	}
	Zero(b)
	for i, v := range b {
		if v != 0 {
			t.Fatalf("应全部为 0: idx=%d", i)
		}
	}
}

func TestNonceUniqueness(t *testing.T) {
	key, _ := RandomKey(256)
	defer Zero(key)
	// 100 次 Seal，nonce 应各不相同
	nonces := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ct, err := SealAESGCM(key, []byte("x"), nil)
		if err != nil {
			t.Fatal(err)
		}
		n := string(ct[:12])
		if nonces[n] {
			t.Fatalf("nonce 重复: %x", ct[:12])
		}
		nonces[n] = true
	}
}

func TestBox(t *testing.T) {
	key, _ := RandomKey(256)
	defer Zero(key)
	b := &Box{Key: key, AAD: []byte("ctx")}
	ct, err := b.Seal([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := b.Open(ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, []byte("hello")) {
		t.Fatal("box roundtrip fail")
	}
}

func TestSealWithNonceExplicit(t *testing.T) {
	key, _ := RandomKey(256)
	defer Zero(key)
	nonce := make([]byte, 12)
	rand.Read(nonce)
	ct, err := SealWithNonce(key, nonce, []byte("x"), nil)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := OpenAESGCM(key, append(append([]byte{}, nonce...), ct...), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, []byte("x")) {
		t.Fatal("mismatch")
	}
}
