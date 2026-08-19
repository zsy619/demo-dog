package cipher

import (
	"bytes"
	"errors"
	"testing"
)

func TestAESGCM_RoundTrip(t *testing.T) {
	key := DeriveKey("secret")
	ct, err := SealAESGCM(key, []byte("hello"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := OpenAESGCM(key, ct, []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pt, []byte("hello")) {
		t.Fatal("roundtrip")
	}
}

func TestAESGCM_WrongKey(t *testing.T) {
	key1 := DeriveKey("k1")
	key2 := DeriveKey("k2")
	ct, _ := SealAESGCM(key1, []byte("x"), nil)
	if _, err := OpenAESGCM(key2, ct, nil); err == nil {
		t.Fatal("应报错")
	}
}

func TestAESGCM_BadKey(t *testing.T) {
	if _, err := SealAESGCM([]byte("short"), []byte("x"), nil); !errors.Is(err, ErrKeyLength) {
		t.Fatal("应 ErrKeyLength")
	}
}

func TestAESGCM_Short(t *testing.T) {
	if _, err := OpenAESGCM(DeriveKey("x"), []byte{1}, nil); !errors.Is(err, ErrShortMessage) {
		t.Fatal("应 ErrShortMessage")
	}
}

func TestBox(t *testing.T) {
	b := &Box{Key: DeriveKey("key")}
	ct, _ := b.Seal([]byte("hello"))
	pt, _ := b.Open(ct)
	if !bytes.Equal(pt, []byte("hello")) {
		t.Fatal("box")
	}
}

func TestDeriveKey(t *testing.T) {
	k := DeriveKey("x")
	if len(k) != 32 {
		t.Fatal("len")
	}
}
