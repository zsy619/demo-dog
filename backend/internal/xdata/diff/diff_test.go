package diff

import (
	"testing"
)

func TestDiff_Map(t *testing.T) {
	a := map[string]any{"x": 1, "y": 2}
	b := map[string]any{"x": 1, "y": 3}
	changes := Diff(a, b)
	if len(changes) != 1 {
		t.Fatal("应只有一个变化")
	}
	if changes[0].Path != "y" {
		t.Fatal("路径错")
	}
}

func TestDiff_Add(t *testing.T) {
	a := map[string]any{"x": 1}
	b := map[string]any{"x": 1, "y": 2}
	changes := Diff(a, b)
	if len(changes) != 1 || changes[0].Op != OpAdd {
		t.Fatal("应为 Add")
	}
}

func TestDiff_Remove(t *testing.T) {
	a := map[string]any{"x": 1, "y": 2}
	b := map[string]any{"x": 1}
	changes := Diff(a, b)
	if len(changes) != 1 || changes[0].Op != OpRemove {
		t.Fatal("应为 Remove")
	}
}

func TestDiff_Struct(t *testing.T) {
	type U struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	a := U{Name: "alice", Age: 30}
	b := U{Name: "alice", Age: 31}
	changes := Diff(a, b)
	if len(changes) != 1 || changes[0].Path != "age" {
		t.Fatal("应只有 age 变化")
	}
}

func TestDiff_Nested(t *testing.T) {
	a := map[string]any{"x": map[string]any{"y": 1}}
	b := map[string]any{"x": map[string]any{"y": 2}}
	changes := Diff(a, b)
	if len(changes) != 1 || changes[0].Path != "x.y" {
		t.Fatal("应使用点路径")
	}
}

func TestDiff_Sorted(t *testing.T) {
	a := map[string]any{"b": 1, "a": 1}
	b := map[string]any{"b": 2, "a": 2}
	changes := Diff(a, b)
	if len(changes) != 2 {
		t.Fatal("应有两个变化")
	}
	if changes[0].Path > changes[1].Path {
		t.Fatal("未排序")
	}
}

func TestHasChanges(t *testing.T) {
	if HasChanges(nil) {
		t.Fatal("nil")
	}
	if !HasChanges([]Change{{Path: "x"}}) {
		t.Fatal("非空")
	}
}
