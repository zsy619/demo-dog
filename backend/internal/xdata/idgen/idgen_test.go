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
	sf, _ := NewSnowflake(1)
	id, _ := sf.Next()
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

func TestSnowflakeInvalidNode(t *testing.T) {
	if _, err := NewSnowflake(2000); err != ErrNodeTooLarge {
		t.Fatal("应 ErrNodeTooLarge")
	}
	if _, err := NewSnowflake(-1); err != ErrNodeTooLarge {
		t.Fatal("应 ErrNodeTooLarge")
	}
}

func TestSnowflakeClockBackward(t *testing.T) {
	sf, _ := NewSnowflake(1)
	// 模拟时钟回拨：手动设置 lastMs > now
	sf.lastMs = time.Now().UnixMilli() + 10000
	if _, err := sf.Next(); err != ErrClockBackward {
		t.Fatal("应 ErrClockBackward")
	}
}

func TestIncCurrent(t *testing.T) {
	g := NewInc(10)
	if g.Current() != 10 {
		t.Fatal("Current")
	}
	g.Next()
	if g.Current() != 11 {
		t.Fatal("Current after Next")
	}
}

func TestDecodeSnowflake(t *testing.T) {
	ts, node, seq := DecodeSnowflake(0, time.Now())
	// ts 应等于 epoch + 0
	if node != 0 || seq != 0 {
		t.Fatal("0 decode node/seq")
	}
	if ts.IsZero() {
		t.Fatal("ts 不应为零")
	}
}
