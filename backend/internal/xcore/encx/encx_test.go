package encx

import "testing"

func TestBase64(t *testing.T) {
	s := Base64Enc([]byte("hi"))
	out, err := Base64Dec(s)
	if err != nil || string(out) != "hi" {
		t.Fatal("b64")
	}
}

func TestBase64URL(t *testing.T) {
	s := Base64URLEnc([]byte("a/b"))
	out, _ := Base64URLDec(s)
	if string(out) != "a/b" {
		t.Fatal("url", s)
	}
}

func TestBase32(t *testing.T) {
	s := Base32Enc([]byte("hi"))
	out, _ := Base32Dec(s)
	if string(out) != "hi" {
		t.Fatal("b32", s)
	}
}

func TestHex(t *testing.T) {
	s := HexEnc([]byte("A"))
	if s != "41" {
		t.Fatal("hex", s)
	}
	out, _ := HexDec(s)
	if string(out) != "A" {
		t.Fatal("hex dec")
	}
}
