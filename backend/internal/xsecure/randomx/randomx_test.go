package randomx

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBytes(t *testing.T) {
	b, err := Bytes(16)
	if err != nil || len(b) != 16 {
		t.Fatal("bytes")
	}
	if _, err := Bytes(0); err != nil { // n<1 应返回 nil
		t.Fatal("0")
	}
}

func TestHex(t *testing.T) {
	h, err := Hex(8)
	if err != nil || len(h) != 16 {
		t.Fatal("hex")
	}
}

func TestBase64(t *testing.T) {
	s, err := Base64(8)
	if err != nil {
		t.Fatal("b64")
	}
	b, _ := base64.StdEncoding.DecodeString(s)
	if len(b) != 8 {
		t.Fatal("b64 len")
	}
}

func TestToken(t *testing.T) {
	tk, err := Token()
	if err != nil || len(tk) == 0 {
		t.Fatal("token")
	}
	tk2, _ := Token(16)
	if len(tk2) == 0 {
		t.Fatal("token 16")
	}
}

func TestInt(t *testing.T) {
	for i := 0; i < 100; i++ {
		v, err := Int(10)
		if err != nil || v < 0 || v >= 10 {
			t.Fatalf("int %d err %v", v, err)
		}
	}
	v, _ := Int(0)
	if v != 0 {
		t.Fatal("max 0")
	}
}

func TestString(t *testing.T) {
	s := String(16)
	if len(s) != 16 {
		t.Fatal("str len")
	}
	// 仅含字母数字
	for _, c := range s {
		if !strings.ContainsRune(alphanumeric, c) {
			t.Fatalf("非字母数字: %q", c)
		}
	}
	if String(0) == "" {
		t.Fatal("0 应返回空或默认 8")
	}
}
