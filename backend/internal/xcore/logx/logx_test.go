package logx

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func newTestLogger(level Level) (*Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return New(buf, level), buf
}

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{
		"trace": LevelTrace,
		"debug": LevelDebug,
		"info":  LevelInfo,
		"warn":  LevelWarn,
		"error": LevelError,
		"fatal": LevelFatal,
		"":      LevelInfo,
		"bogus": LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("%q: %d want %d", in, got, want)
		}
	}
}

func TestLevel_String(t *testing.T) {
	for _, l := range []Level{LevelTrace, LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal, Level(99)} {
		if l.String() == "unknown" && l != Level(99) {
			t.Fatalf("%d: %s", l, l)
		}
	}
}

func TestLogger_EmitsJSON(t *testing.T) {
	l, buf := newTestLogger(LevelInfo)
	l.Info("hello", Str("name", "alice"), Int("count", 3))
	line := buf.String()
	if !strings.HasSuffix(line, "\n") {
		t.Fatal("missing trailing newline")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &m); err != nil {
		t.Fatalf("bad json: %v (%q)", err, line)
	}
	if m["msg"] != "hello" || m["name"] != "alice" || m["count"].(float64) != 3 {
		t.Fatalf("fields: %v", m)
	}
	if m["level"] != "info" {
		t.Fatal("level")
	}
}

func TestLogger_LevelFilter(t *testing.T) {
	l, buf := newTestLogger(LevelWarn)
	l.Info("skip")
	l.Warn("keep")
	out := buf.String()
	if strings.Contains(out, "skip") {
		t.Fatal("info should be filtered")
	}
	if !strings.Contains(out, "keep") {
		t.Fatal("warn should pass")
	}
}

func TestLogger_With(t *testing.T) {
	l, buf := newTestLogger(LevelInfo)
	child := l.With(Str("tenant", "acme"), Int("region", 3))
	child.Info("msg")
	var m map[string]any
	json.Unmarshal(buf.Bytes(), &m)
	if m["tenant"] != "acme" || m["region"].(float64) != 3 {
		t.Fatalf("with fields missing: %v", m)
	}
}

func TestLogger_With_DoesNotLeak(t *testing.T) {
	l, buf := newTestLogger(LevelInfo)
	child := l.With(Str("a", "x"))
	l.Info("parent")
	child.Info("child")
	out := buf.String()
	parent := strings.Split(strings.Split(out, "\n")[0], "")
	if strings.Contains(strings.Join(parent, ""), `"a"`) {
		t.Fatalf("parent should not inherit child field: %s", out)
	}
}

func TestLogger_AllFieldKinds(t *testing.T) {
	l, buf := newTestLogger(LevelInfo)
	l.Info("e",
		Str("s", "x"),
		Int("i", 5),
		Int64("i64", 1<<40),
		Bool("b", true),
		Dur("d", 2*time.Second),
		Err(errors.New("boom")),
		Time("t", time.Unix(1700000000, 0).UTC()),
		Any("any", []string{"a", "b"}),
	)
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["s"] != "x" {
		t.Fatal("string")
	}
	if m["b"] != true {
		t.Fatal("bool")
	}
	if m["error"] != "boom" {
		t.Fatal("err")
	}
}

func TestLogger_Err_Nil(t *testing.T) {
	f := Err(nil)
	if f.Value != nil {
		t.Fatal("nil err should be nil")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	l, buf := newTestLogger(LevelError)
	l.Info("drop")
	l.SetLevel(LevelDebug)
	l.Info("keep")
	if !strings.Contains(buf.String(), "keep") {
		t.Fatal("after SetLevel should pass")
	}
}

func TestLogger_Default(t *testing.T) {
	if Default == nil {
		t.Fatal("Default should be set")
	}
}

func TestLogger_ConcurrentSafe(t *testing.T) {
	l, buf := newTestLogger(LevelInfo)
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				l.Info("concurrent", Int("i", j))
			}
			done <- true
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	if buf.Len() == 0 {
		t.Fatal("expected output")
	}
	// 校验每一行都能解析。
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var x map[string]any
		if err := json.Unmarshal([]byte(line), &x); err != nil {
			t.Fatalf("bad line: %v", err)
		}
	}
}

func TestLogger_CallerField(t *testing.T) {
	l, buf := newTestLogger(LevelInfo)
	l.Info("e", Caller(0))
	var m map[string]any
	json.Unmarshal(buf.Bytes(), &m)
	c, ok := m["caller"].(string)
	if !ok || !strings.Contains(c, "logx_test.go") {
		t.Fatalf("caller: %v", m["caller"])
	}
}

func TestLogger_Stats(t *testing.T) {
	l, _ := newTestLogger(LevelDebug)
	child := l.With(Str("x", "y"))
	s := child.Stats()
	if s.MinLevel != LevelDebug || s.Fields != 1 {
		t.Fatalf("stats: %+v", s)
	}
}

func TestJSONEncoder_NilErr(t *testing.T) {
	r := &Record{Time: time.Now(), Level: LevelInfo, Message: "x", Fields: []Field{Err(nil)}}
	b, err := JSONEncoder{}.Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["error"] != nil {
		t.Fatal("nil err should produce null")
	}
}

func TestJSONEncoder_AllFields(t *testing.T) {
	r := &Record{
		Time: time.Unix(1700000000, 0).UTC(),
		Level: LevelInfo,
		Message: "m",
		Fields: []Field{Str("k", "v")},
	}
	b, err := JSONEncoder{}.Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["msg"] != "m" || m["k"] != "v" {
		t.Fatalf("%+v", m)
	}
}
