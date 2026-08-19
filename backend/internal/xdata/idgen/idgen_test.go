package idgen

import (
	"testing"
	"time"
)

func TestInc(t *testing.T) {
	g := NewInc(0)
	if g.Next() != 1 {
		t.Fatal("first")
	}
	if g.Next() != 2 {
		t.Fatal("second")
	}
}

func TestRandom(t *testing.T) {
	g := NewRandom()
	h := g.Hex(8)
	if len(h) != 16 {
		t.Fatal("hex", h)
	}
	b := g.B64(8)
	if b == "" {
		t.Fatal("b64")
	}
}

func TestSnowflake(t *testing.T) {
	sf := NewSnowflake(1)
	id := sf.Next()
	if id == 0 {
		t.Fatal("sf")
	}
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_, n, seq := DecodeSnowflake(id, epoch)
	if n != 1 || seq != 0 {
		t.Fatal("decode", n, seq)
	}
}

func TestShortID(t *testing.T) {
	s := ShortID()
	if len(s) != 10 {
		t.Fatal("short", s)
	}
}
