package ed25519x

import "testing"

func TestGenerate(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(kp.Priv) != 64 || len(kp.Pub) != 32 {
		t.Fatal("sizes", len(kp.Priv), len(kp.Pub))
	}
}

func TestRoundTrip(t *testing.T) {
	kp, _ := Generate()
	msg := []byte("hello ed25519")
	sig, err := Sign(kp.Priv, msg)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify(kp.Pub, msg, sig)
	if err != nil || !ok {
		t.Fatal("verify")
	}
}

func TestWrongSig(t *testing.T) {
	kp, _ := Generate()
	msg := []byte("a")
	sig, _ := Sign(kp.Priv, msg)
	sig[0] ^= 0xff
	ok, _ := Verify(kp.Pub, msg, sig)
	if ok {
		t.Fatal("应失败")
	}
}

func TestBadPriv(t *testing.T) {
	if _, err := Sign([]byte("short"), []byte("x")); err == nil {
		t.Fatal("bad priv")
	}
}

func TestBadPub(t *testing.T) {
	if _, err := Verify([]byte("short"), []byte("x"), []byte("sig")); err == nil {
		t.Fatal("bad pub")
	}
}

func TestString(t *testing.T) {
	kp, _ := Generate()
	sig, _ := SignString(kp.Priv, "hi")
	ok, _ := VerifyString(kp.Pub, sig, "hi")
	if !ok {
		t.Fatal("str")
	}
}
