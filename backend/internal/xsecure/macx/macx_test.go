package macx

import "testing"

func TestSignVerify(t *testing.T) {
	k := []byte("key")
	msg := []byte("hello")
	s, err := Sign(SHA256, k, msg)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(SHA256, k, msg, s)
	if err != nil || !ok {
		t.Fatal("verify")
	}
}

func TestBadAlg(t *testing.T) {
	if _, err := Sign(Alg("xxx"), []byte("k"), []byte("m")); err == nil {
		t.Fatal("bad alg")
	}
}

func TestTruncated(t *testing.T) {
	s, err := Truncated(SHA256, []byte("k"), []byte("m"), 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 16 { // 8 bytes -> 16 hex
		t.Fatal("trunc", len(s))
	}
}

func TestTruncatedZero(t *testing.T) {
	if _, err := Truncated(SHA256, []byte("k"), []byte("m"), 0); err == nil {
		t.Fatal("zero")
	}
}

func TestSHA1(t *testing.T) {
	s, _ := Sign(SHA1, []byte("k"), []byte("m"))
	if len(s) != 40 {
		t.Fatal("sha1", len(s))
	}
}

func TestSHA512(t *testing.T) {
	s, _ := Sign(SHA512, []byte("k"), []byte("m"))
	if len(s) != 128 {
		t.Fatal("sha512", len(s))
	}
}
