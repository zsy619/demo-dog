package loggerx

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	l := New()
	l.SetOutput(&buf)
	l.Info("hello", map[string]any{"a": 1})
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["msg"] != "hello" || m["level"] != "info" {
		t.Fatal("hdr")
	}
}

func TestLevel(t *testing.T) {
	l := New()
	l.SetLevel(LevelWarn)
	if l.level != LevelWarn {
		t.Fatal("set")
	}
}

func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	l := New()
	l.SetOutput(&buf)
	l.SetLevel(LevelError)
	l.Info("ignored", nil)
	l.Debug("ignored2", nil)
	l.Error("kept", nil)
	if !strings.Contains(buf.String(), "kept") {
		t.Fatal("filter")
	}
	if strings.Contains(buf.String(), "ignored") {
		t.Fatal("应忽略")
	}
}

func TestLevelString(t *testing.T) {
	if LevelDebug.String() != "debug" {
		t.Fatal("str")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	l := New()
	l.SetOutput(&buf)
	sub := l.With(map[string]any{"svc": "x"})
	sub.Info("hi", nil)
	if !strings.Contains(buf.String(), "\"svc\":\"x\"") {
		t.Fatal("with")
	}
}
