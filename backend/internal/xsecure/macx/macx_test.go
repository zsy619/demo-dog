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

func TestVerifyBytes(t *testing.T) {
	key := []byte("secret-key-at-least-16-bytes")
	msg := []byte("hello")
	mac, err := SignBytes(SHA256, key, msg)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := VerifyBytes(SHA256, key, msg, mac); !ok {
		t.Fatal("应通过")
	}
	// 篡改最后字节
	bad := append([]byte{}, mac...)
	bad[len(bad)-1] ^= 0xff
	if ok, _ := VerifyBytes(SHA256, key, msg, bad); ok {
		t.Fatal("应失败")
	}
}

func TestUnknownAlg(t *testing.T) {
	if _, err := Sign("md5", []byte("k"), []byte("m")); err != ErrUnknownAlg {
		t.Fatal("应 ErrUnknownAlg")
	}
}

func TestSize(t *testing.T) {
	s, err := Size(SHA256)
	if err != nil || s != 32 {
		t.Fatal("SHA256 size", s, err)
	}
	s, err = Size(SHA512)
	if err != nil || s != 64 {
		t.Fatal("SHA512 size", s, err)
	}
}

func TestCheckKey(t *testing.T) {
	if err := CheckKey([]byte("short"), true); err != ErrShortKey {
		t.Fatal("weak=true 应拒绝短密钥")
	}
	if err := CheckKey([]byte("short"), false); err != nil {
		t.Fatal("weak=false 应通过")
	}
}

func TestVerifyCorrupted(t *testing.T) {
	key := []byte("secret-key-at-least-16-bytes")
	mac, _ := Sign(SHA256, key, []byte("m"))
	// 篡改
	bad := mac[:len(mac)-1] + "0"
	if ok, _ := Verify(SHA256, key, []byte("m"), bad); ok {
		t.Fatal("应失败")
	}
}

func TestTruncatedBounds(t *testing.T) {
	key := []byte("secret-key-at-least-16-bytes")
	// n > size 自动截断
	if _, err := Truncated(SHA256, key, []byte("m"), 100); err != nil {
		t.Fatal("truncated")
	}
	if _, err := Truncated(SHA256, key, []byte("m"), 0); err != ErrInvalidN {
		t.Fatal("应 ErrInvalidN")
	}
}
