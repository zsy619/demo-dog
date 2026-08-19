package hmac

import "testing"

func TestSHA256(t *testing.T) {
	s := SHA256([]byte("key"), []byte("msg"))
	if len(s) != 64 {
		t.Fatal("len", s)
	}
}

func TestSHA1(t *testing.T) {
	s := SHA1([]byte("key"), []byte("msg"))
	if len(s) != 40 {
		t.Fatal("len", s)
	}
}

func TestSHA512(t *testing.T) {
	s := SHA512([]byte("key"), []byte("msg"))
	if len(s) != 128 {
		t.Fatal("len", s)
	}
}

func TestSHA256B64(t *testing.T) {
	s := SHA256B64([]byte("key"), []byte("msg"))
	if s == "" {
		t.Fatal("b64")
	}
}

func TestVerifySHA256(t *testing.T) {
	k := []byte("key")
	m := []byte("hello")
	s := SHA256(k, m)
	if !VerifySHA256(k, m, s) {
		t.Fatal("verify")
	}
	if VerifySHA256(k, m, "bad") {
		t.Fatal("bad verify")
	}
}
