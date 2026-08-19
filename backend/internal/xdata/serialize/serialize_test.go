package serialize

import (
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	h := Header{Magic: DefaultMagic, Version: 1, Flags: 2}
	data := []byte("hello")
	buf := Encode(data, h)
	h2, got, err := Decode(buf, DefaultMagic)
	if err != nil {
		t.Fatal(err)
	}
	if h2.Version != 1 || h2.Flags != 2 {
		t.Fatal("hdr")
	}
	if string(got) != "hello" {
		t.Fatal("data")
	}
}

func TestDecode_BadMagic(t *testing.T) {
	h := Header{Magic: DefaultMagic}
	buf := Encode([]byte("x"), h)
	if _, _, err := Decode(buf, 0x12345678); !errors.Is(err, ErrBadMagic) {
		t.Fatal("应 ErrBadMagic")
	}
}

func TestDecode_Short(t *testing.T) {
	if _, _, err := Decode([]byte("abc"), DefaultMagic); err == nil {
		t.Fatal("应报错")
	}
}

func TestUint64(t *testing.T) {
	b := Uint64(0x1122334455667788)
	if len(b) != 8 {
		t.Fatal("len")
	}
}

func TestUint32(t *testing.T) {
	b := Uint32(0x11223344)
	if len(b) != 4 {
		t.Fatal("len")
	}
}
