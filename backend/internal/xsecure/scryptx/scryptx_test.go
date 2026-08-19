package scryptx

import "testing"

func TestHashVerify(t *testing.T) {
	h, err := Hash("mypassword")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(h, "mypassword")
	if err != nil || !ok {
		t.Fatal("verify")
	}
}

func TestWrong(t *testing.T) {
	h, _ := Hash("right")
	ok, _ := Verify(h, "wrong")
	if ok {
		t.Fatal("应失败")
	}
}

func TestBadFormat(t *testing.T) {
	if _, err := Verify("bad-format", "x"); err == nil {
		t.Fatal("应报错")
	}
}
