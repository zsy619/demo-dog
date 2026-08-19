package password

import "testing"

func TestScore(t *testing.T) {
	if Score("abc") != StrengthWeak {
		t.Fatal("weak")
	}
	if Score("Abc12345") == StrengthWeak {
		t.Fatal("fair 期望")
	}
	if Score("Password123") == StrengthStrong {
		t.Fatal("Password123 应 good")
	}
	if Score("Abcdef1!SuperLongPassword") != StrengthStrong {
		t.Fatal("strong 期望")
	}
}

func TestCommon(t *testing.T) {
	if !isCommon("password") {
		t.Fatal("common")
	}
}

func TestScore_Common(t *testing.T) {
	if Score("password") != StrengthWeak {
		t.Fatal("common 应降级")
	}
}

func TestStrengthString(t *testing.T) {
	if StrengthStrong.String() != "strong" {
		t.Fatal("str")
	}
}

func TestHashVerify(t *testing.T) {
	p := "MyStrongPass!1"
	h, err := Hash(p, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Fatal("hash")
	}
	ok, err := Verify(h, p)
	if err != nil || !ok {
		t.Fatal("verify")
	}
}

func TestVerify_Wrong(t *testing.T) {
	h, _ := Hash("a", 1000)
	ok, _ := Verify(h, "b")
	if ok {
		t.Fatal("wrong 应拒")
	}
}

func TestVerify_BadFormat(t *testing.T) {
	if _, err := Verify("abc", "x"); err == nil {
		t.Fatal("应报错")
	}
}
