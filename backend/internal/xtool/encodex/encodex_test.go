package encodex

import (
	"bytes"
	"testing"
)

func TestBase64RoundTrip(t *testing.T) {
	in := []byte("hello world")
	s := Base64Std(in)
	out, err := FromBase64Std(s)
	if err != nil || !bytes.Equal(out, in) {
		t.Fatal("b64")
	}
}

func TestBase64URL(t *testing.T) {
	in := []byte("abc~")
	s := Base64URL(in)
	out, _ := FromBase64URL(s)
	if !bytes.Equal(out, in) {
		t.Fatal("b64url")
	}
}

func TestHex(t *testing.T) {
	in := []byte{0x01, 0xfe, 0x33}
	s := Hex(in)
	out, err := FromHex(s)
	if err != nil || !bytes.Equal(out, in) {
		t.Fatal("hex")
	}
}

func TestBase32(t *testing.T) {
	in := []byte("hello")
	s := Base32Std(in)
	out, err := FromBase32Std(s)
	if err != nil || !bytes.Equal(out, in) {
		t.Fatal("b32")
	}
}

func TestBase58(t *testing.T) {
	in := []byte("hello world")
	s := Base58(in)
	out, err := FromBase58(s)
	if err != nil || !bytes.Equal(out, in) {
		t.Fatal("b58")
	}
}

func TestBase58Zeros(t *testing.T) {
	in := []byte{0, 0, 0, 'h'}
	s := Base58(in)
	if s[:3] != "111" {
		t.Fatal("zeros")
	}
	out, err := FromBase58(s)
	if err != nil || !bytes.Equal(out, in) {
		t.Fatal("b58z", out)
	}
}

func TestFromBase58_Bad(t *testing.T) {
	if _, err := FromBase58("0!@#"); err == nil {
		t.Fatal("应报错")
	}
}
