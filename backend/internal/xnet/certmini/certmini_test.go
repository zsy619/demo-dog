package certmini

import "testing"

func TestParseEncode(t *testing.T) {
	orig := []byte("hello")
	p := EncodePEM("TEST", orig)
	if len(p) == 0 {
		t.Fatal("enc")
	}
	typ, b, err := ParsePEM(p)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "TEST" || string(b) != "hello" {
		t.Fatal("round")
	}
}

func TestParseBad(t *testing.T) {
	if _, _, err := ParsePEM([]byte("garbage")); err == nil {
		t.Fatal("bad")
	}
}

func TestSelfSigned(t *testing.T) {
	c, k, err := SelfSigned("test", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) == 0 || len(k) == 0 {
		t.Fatal("empty")
	}
}
