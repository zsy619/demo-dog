package snapshot

import (
	"errors"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	v := map[string]any{"hello": "world", "n": 42}
	data, err := Encode(v, "测试")
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	meta, err := Decode(data, &out)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Version != FormatVersion {
		t.Fatal("版本")
	}
	if out["hello"] != "world" {
		t.Fatal("值")
	}
}

func TestBadMagic(t *testing.T) {
	var v any
	if _, err := Decode([]byte("BADMAGIC"), &v); !errors.Is(err, ErrBadMagic) {
		t.Fatal("应 ErrBadMagic")
	}
}

func TestBadChecksum(t *testing.T) {
	data, _ := Encode(map[string]any{}, "")
	data[10] ^= 0xFF // 翻转一个字节
	var v any
	if _, err := Decode(data, &v); !errors.Is(err, ErrBadChecksum) {
		t.Fatal("应 ErrBadChecksum")
	}
}

func TestChecksum(t *testing.T) {
	if Checksum([]byte("abc")) == "" {
		t.Fatal("checksum")
	}
}

func TestEmpty(t *testing.T) {
	data, _ := Encode(map[string]any{}, "")
	var v map[string]any
	if _, err := Decode(data, &v); err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Fatal("空应为空")
	}
}

func TestRoundTrip_Slice(t *testing.T) {
	data, _ := Encode([]int{1, 2, 3}, "")
	var out []int
	if _, err := Decode(data, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 || out[0] != 1 || out[1] != 2 || out[2] != 3 {
		t.Fatal("slice")
	}
}
