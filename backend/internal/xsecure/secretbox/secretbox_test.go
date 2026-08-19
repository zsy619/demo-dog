package secretbox

import (
	"bytes"
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	k, _ := RandomKey()
	pt := []byte("my secret")
	ct, err := Wrap(k, pt)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct, pt) {
		t.Fatal("应被加密")
	}
	out, err := Unwrap(k, ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, pt) {
		t.Fatal("roundtrip")
	}
}

func TestBadKey(t *testing.T) {
	if _, err := Wrap([]byte("short"), []byte("x")); !errors.Is(err, ErrBadSize) {
		t.Fatal("应 ErrBadSize")
	}
}

func TestOpenFail(t *testing.T) {
	k, _ := RandomKey()
	pt := []byte("data")
	ct, _ := Wrap(k, pt)
	if _, err := Unwrap([]byte("1234567890123456"), ct); !errors.Is(err, ErrOpen) {
		t.Fatal("应 ErrOpen")
	}
}

func TestOpen_Short(t *testing.T) {
	k, _ := RandomKey()
	if _, err := Unwrap(k, []byte("abc")); !errors.Is(err, ErrOpen) {
		t.Fatal("应 ErrOpen")
	}
}

func TestRandomKey(t *testing.T) {
	k, err := RandomKey()
	if err != nil || len(k) != 32 {
		t.Fatal("rk")
	}
}
