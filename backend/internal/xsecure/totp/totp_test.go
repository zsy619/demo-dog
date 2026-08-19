package totp

import (
	"testing"
	"time"
)

func TestGenerate_Stable(t *testing.T) {
	c := Config{Secret: []byte("12345678901234567890")}
	t1 := time.Unix(59, 0)
	code := Generate(c, t1)
	if len(code) != 6 {
		t.Fatal("digits")
	}
	if code != Generate(c, t1) {
		t.Fatal("stable")
	}
}

func TestVerify_Window(t *testing.T) {
	c := Config{Secret: []byte("12345678901234567890")}
	t1 := time.Unix(59, 0)
	code := Generate(c, t1)
	if !Verify(c, t1, code, 0) {
		t.Fatal("应通过")
	}
}

func TestVerify_Reject(t *testing.T) {
	c := Config{Secret: []byte("12345678901234567890")}
	t1 := time.Unix(59, 0)
	if Verify(c, t1, "000000", 0) {
		t.Fatal("应拒绝")
	}
}

func TestVerify_WrongDigits(t *testing.T) {
	c := Config{Secret: []byte("x"), Digits: 8}
	t1 := time.Unix(0, 0)
	if Verify(c, t1, "123456", 0) {
		t.Fatal("应拒绝位数错误")
	}
}

func TestDifferentTimeSteps(t *testing.T) {
	c := Config{Secret: []byte("k")}
	a := Generate(c, time.Unix(0, 0))
	b := Generate(c, time.Unix(60, 0))
	if a == b {
		t.Fatal("不同步骤应不同")
	}
}

func TestLeftPad(t *testing.T) {
	if leftPad(0, 6) != "000000" {
		t.Fatal("0")
	}
	if leftPad(123, 6) != "000123" {
		t.Fatal("pad")
	}
}

func TestSHA256(t *testing.T) {
	c := Config{Secret: []byte("x"), Algorithm: SHA256}
	t1 := time.Unix(0, 0)
	code := Generate(c, t1)
	if len(code) != 6 {
		t.Fatal("sha256")
	}
}
