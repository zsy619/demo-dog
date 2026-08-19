package printx

import (
	"bytes"
	"strings"
	"testing"
)

func TestKV(t *testing.T) {
	var buf bytes.Buffer
	KV(&buf, map[string]any{"a": 1, "b": "x"})
	if !strings.Contains(buf.String(), "a=1") || !strings.Contains(buf.String(), "b=x") {
		t.Fatal("kv")
	}
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, [][]string{
		{"A", "B"},
		{"1", "22"},
		{"333", "4"},
	})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatal("rows")
	}
}

func TestTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, nil)
	if buf.Len() != 0 {
		t.Fatal("empty")
	}
}

func TestList(t *testing.T) {
	var buf bytes.Buffer
	List(&buf, []string{"a", "b"})
	if !strings.Contains(buf.String(), "1. a") || !strings.Contains(buf.String(), "2. b") {
		t.Fatal("list")
	}
}

func TestSection(t *testing.T) {
	var buf bytes.Buffer
	Section(&buf, "Title")
	if !strings.Contains(buf.String(), "== Title ==") {
		t.Fatal("section")
	}
}

func TestIndent(t *testing.T) {
	var buf bytes.Buffer
	Indent(&buf, "hi\nbye", 2)
	if !strings.Contains(buf.String(), "    hi") || !strings.Contains(buf.String(), "    bye") {
		t.Fatal("indent")
	}
}

func TestBytes(t *testing.T) {
	if Bytes(100) != "100B" {
		t.Fatal("B")
	}
	if !strings.Contains(Bytes(2048), "KB") {
		t.Fatal("KB")
	}
}

func TestDuration(t *testing.T) {
	if Duration(500) != "500ns" {
		t.Fatal("ns")
	}
	if !strings.Contains(Duration(1500000), "ms") {
		t.Fatal("ms")
	}
}
