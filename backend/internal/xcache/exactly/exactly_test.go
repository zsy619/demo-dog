package exactly

import "testing"

func TestMarkOnce(t *testing.T) {
	m := New()
	if !m.Mark("a") {
		t.Fatal("first")
	}
	if m.Mark("a") {
		t.Fatal("second 应失败")
	}
}

func TestUnmark(t *testing.T) {
	m := New()
	m.Mark("a")
	m.Unmark("a")
	if !m.Mark("a") {
		t.Fatal("unmark 后应允许")
	}
}

func TestIsMarked(t *testing.T) {
	m := New()
	if m.IsMarked("a") {
		t.Fatal("不应标记")
	}
	m.Mark("a")
	if !m.IsMarked("a") {
		t.Fatal("应标记")
	}
}

func TestClear(t *testing.T) {
	m := New()
	m.Mark("a")
	m.Clear()
	if m.IsMarked("a") {
		t.Fatal("clear")
	}
}

func TestLen(t *testing.T) {
	m := New()
	m.Mark("a")
	m.Mark("b")
	if m.Len() != 2 {
		t.Fatal("len")
	}
}
