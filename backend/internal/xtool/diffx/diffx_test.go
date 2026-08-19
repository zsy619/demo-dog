package diffx

import (
	"strings"
	"testing"
)

func TestDiff_Identical(t *testing.T) {
	d := DiffLine("a\nb\nc", "a\nb\nc")
	if len(d) != 3 {
		t.Fatal("len")
	}
	for _, c := range d {
		if c.Op != OpEq {
			t.Fatal("应 eq")
		}
	}
}

func TestDiff_AddDel(t *testing.T) {
	d := DiffLine("a\nb", "a\nc\nb")
	add, del := Stat(d)
	if add != 1 || del != 0 {
		t.Fatal("stat")
	}
}

func TestDiff_Replace(t *testing.T) {
	d := DiffLine("a\nb", "a\nc")
	add, del := Stat(d)
	if add != 1 || del != 1 {
		t.Fatal("replace", add, del)
	}
}

func TestUnified(t *testing.T) {
	d := DiffLine("a\nb", "a\nc")
	u := Unified(d)
	if !strings.Contains(u, "+ c") || !strings.Contains(u, "- b") {
		t.Fatal("unified")
	}
}

func TestStat(t *testing.T) {
	add, del := Stat(nil)
	if add != 0 || del != 0 {
		t.Fatal("empty")
	}
}
