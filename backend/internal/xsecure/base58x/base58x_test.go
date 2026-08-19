package base58x

import "testing"

func TestRoundTrip(t *testing.T) {
	in := []byte("hello world")
	s := Encode(in)
	out, err := Decode(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(in) {
		t.Fatal("rt")
	}
}

func TestEmpty(t *testing.T) {
	if Encode(nil) != "" {
		t.Fatal("empty enc")
	}
	out, err := Decode("")
	if err != nil || out != nil {
		t.Fatal("empty dec")
	}
}

func TestLeadingZero(t *testing.T) {
	in := []byte{0, 0, 1, 2}
	s := Encode(in)
	if s == "" {
		t.Fatal("lz enc")
	}
	out, err := Decode(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) || out[0] != 0 || out[2] != 1 {
		t.Fatal("lz dec", out)
	}
}

func TestInvalidChar(t *testing.T) {
	if _, err := Decode("0OIl"); err == nil {
		t.Fatal("invalid")
	}
}
