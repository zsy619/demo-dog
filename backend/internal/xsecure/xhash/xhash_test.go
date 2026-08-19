package xhash

import "testing"

func TestMD5(t *testing.T) {
	if MD5Hex([]byte("")) != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatal("md5")
	}
}

func TestSHA1(t *testing.T) {
	if len(SHA1Hex([]byte("a"))) != 40 {
		t.Fatal("sha1")
	}
}

func TestSHA256(t *testing.T) {
	if len(SHA256Hex([]byte("a"))) != 64 {
		t.Fatal("sha256")
	}
}

func TestSHA512(t *testing.T) {
	if len(SHA512Hex([]byte("a"))) != 128 {
		t.Fatal("sha512")
	}
}

func TestFNV(t *testing.T) {
	if FNV32([]byte("a")) == 0 {
		t.Fatal("fnv32")
	}
	if FNV64([]byte("a")) == 0 {
		t.Fatal("fnv64")
	}
}

func TestString(t *testing.T) {
	if StringSHA256("hi") != SHA256Hex([]byte("hi")) {
		t.Fatal("str")
	}
	if StringMD5("hi") != MD5Hex([]byte("hi")) {
		t.Fatal("md5 str")
	}
}
