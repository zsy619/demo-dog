package strutilx

import "testing"

func TestTruncate(t *testing.T) {
	if Truncate("你好世界", 2) != "你好" {
		t.Fatal("trunc")
	}
	if Truncate("hi", 10) != "hi" {
		t.Fatal("short")
	}
}

func TestPad(t *testing.T) {
	if PadLeft("x", "0", 4) != "000x" {
		t.Fatal("left")
	}
	if PadRight("x", "0", 4) != "x000" {
		t.Fatal("right")
	}
}

func TestLines(t *testing.T) {
	lines := SplitLines("a\nb\nc")
	if len(lines) != 3 {
		t.Fatal("lines", lines)
	}
}

func TestWords(t *testing.T) {
	ws := Words("  a b	c  ")
	if len(ws) != 3 {
		t.Fatal("words", ws)
	}
}

func TestReverse(t *testing.T) {
	if Reverse("hello") != "olleh" {
		t.Fatal("rev")
	}
	if Reverse("你好") != "好你" {
		t.Fatal("rev 中文")
	}
}

func TestContainsAny(t *testing.T) {
	if !ContainsAny("foo bar", "x", "bar") {
		t.Fatal("any")
	}
	if ContainsAny("foo", "x", "y") {
		t.Fatal("none")
	}
}

func TestCount(t *testing.T) {
	if CountOccurrences("aabbaa", "a") != 4 {
		t.Fatal("count")
	}
}
