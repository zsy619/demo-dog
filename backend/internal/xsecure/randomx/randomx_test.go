package randomx

import "testing"

func TestBytes(t *testing.T) {
	b, err := Bytes(16)
	if err != nil || len(b) != 16 {
		t.Fatal("b", len(b))
	}
}

func TestHex(t *testing.T) {
	s, err := Hex(8)
	if err != nil || len(s) != 16 {
		t.Fatal("hex", len(s))
	}
}

func TestBase64(t *testing.T) {
	s, _ := Base64(8)
	if len(s) == 0 {
		t.Fatal("b64")
	}
}

func TestInt(t *testing.T) {
	for i := 0; i < 100; i++ {
		v, err := Int(10)
		if err != nil || v < 0 || v >= 10 {
			t.Fatal("int", v)
		}
	}
}

func TestToken(t *testing.T) {
	t0, _ := Token()
	t1, _ := Token()
	if t0 == t1 {
		t.Fatal("重复")
	}
}

func TestBytesZero(t *testing.T) {
	b, _ := Bytes(0)
	if len(b) != 0 {
		t.Fatal("0")
	}
}
