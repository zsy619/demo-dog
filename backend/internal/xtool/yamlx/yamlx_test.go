package yamlx

import (
	"bytes"
	"strings"
	"testing"
)

func TestMarshal_Map(t *testing.T) {
	var buf bytes.Buffer
	Marshal(&buf, map[string]any{"a": 1, "b": "x"})
	out := buf.String()
	if !strings.Contains(out, "a: 1") || !strings.Contains(out, "b: x") {
		t.Fatal("map")
	}
}

func TestMarshal_Nested(t *testing.T) {
	var buf bytes.Buffer
	Marshal(&buf, map[string]any{"a": map[string]any{"b": 1}})
	out := buf.String()
	if !strings.Contains(out, "a:") || !strings.Contains(out, "b: 1") {
		t.Fatal("nested")
	}
}

func TestMarshal_List(t *testing.T) {
	var buf bytes.Buffer
	Marshal(&buf, map[string]any{"list": []any{1, 2, 3}})
	out := buf.String()
	if !strings.Contains(out, "-") {
		t.Fatal("list")
	}
}

func TestMarshal_Nil(t *testing.T) {
	var buf bytes.Buffer
	Marshal(&buf, nil)
	if !strings.Contains(buf.String(), "null") {
		t.Fatal("nil")
	}
}

func TestMarshal_Bool(t *testing.T) {
	var buf bytes.Buffer
	Marshal(&buf, true)
	if !strings.Contains(buf.String(), "true") {
		t.Fatal("bool")
	}
}
