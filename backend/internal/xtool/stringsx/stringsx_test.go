package stringsx

import "testing"

func TestJoin(t *testing.T) {
	if Join([]string{"a", "b", "c"}, ",") != "a,b,c" {
		t.Fatal("join")
	}
}

func TestBuilder(t *testing.T) {
	b := NewBuilder()
	b.Write("a").WriteInt(42).WriteByte('!')
	if b.String() != "a42!" {
		t.Fatal("builder", b.String())
	}
}

func TestBuilder_Zero(t *testing.T) {
	b := NewBuilder()
	b.WriteInt(0)
	if b.String() != "0" {
		t.Fatal("zero")
	}
}

func TestBuilder_Negative(t *testing.T) {
	b := NewBuilder()
	b.WriteInt(-5)
	if b.String() != "-5" {
		t.Fatal("neg", b.String())
	}
}

func TestBuilder_Hex(t *testing.T) {
	b := NewBuilder()
	b.WriteByteArray([]byte{0xAB, 0x0C})
	if b.String() != "ab0c" {
		t.Fatal("hex", b.String())
	}
}

func TestBuilder_Reset(t *testing.T) {
	b := NewBuilder()
	b.Write("abc")
	b.Reset()
	if b.Len() != 0 {
		t.Fatal("reset")
	}
}
