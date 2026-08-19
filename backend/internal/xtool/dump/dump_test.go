package dump

import (
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	s := JSON(map[string]int{"a": 1})
	if !strings.Contains(s, `"a":1`) {
		t.Fatal("json")
	}
}

func TestJSONIndent(t *testing.T) {
	s := JSONIndent(map[string]int{"a": 1})
	if !strings.Contains(s, "\n") {
		t.Fatal("indent")
	}
}

func TestKV(t *testing.T) {
	s := KV(map[string]any{"a": 1, "b": "x"})
	if !strings.Contains(s, "a=1") || !strings.Contains(s, "b=x") {
		t.Fatal("kv")
	}
}

func TestTable(t *testing.T) {
	s := Table([]string{"A", "B"}, [][]string{{"1", "2"}, {"3", "4"}})
	if !strings.Contains(s, "A") || !strings.Contains(s, "1") {
		t.Fatal("table")
	}
}

func TestPretty(t *testing.T) {
	type S struct {
		X int
	}
	s := Pretty(S{X: 1})
	if !strings.Contains(s, "S") {
		t.Fatal("pretty")
	}
}
