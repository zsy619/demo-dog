package jsonx

import (
	"bytes"
	"errors"
	"testing"
)

func TestWriter_Object(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Object()
	w.Key("a")
	w.Int(1)
	w.Comma()
	w.Key("b")
	w.String("x")
	w.EndObject()
	if buf.String() != `{"a":1,"b":"x"}` {
		t.Fatal(buf.String())
	}
}

func TestWriter_Array(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Array()
	for i, v := range []int{1, 2, 3} {
		if i > 0 {
			w.Comma()
		}
		w.Int(int64(v))
	}
	w.EndArray()
	if buf.String() != "[1,2,3]" {
		t.Fatal(buf.String())
	}
}

func TestWriter_StringEscape(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.String("a\"b\\c\nd")
	if buf.String() != `"a\"b\\c\nd"` {
		t.Fatal(buf.String())
	}
}

func TestWriter_BoolNull(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Bool(true)
	w.Comma()
	w.Bool(false)
	w.Comma()
	w.Null()
	if buf.String() != "true,false,null" {
		t.Fatal(buf.String())
	}
}

func TestWriter_Float(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Float(3.14)
	if buf.String() != "3.14" {
		t.Fatal(buf.String())
	}
}

func TestWriter_ErrPropagates(t *testing.T) {
	w := NewWriter(errorWriter{})
	w.Int(1)
	if w.Err() == nil {
		t.Fatal("expected error")
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("io err") }

func TestReader_Basic(t *testing.T) {
	r := NewReader([]byte(`{"a":1,"b":"x"}`))
	t1, _ := r.Next()
	if t1.Type != TokenObjectStart {
		t.Fatal("start")
	}
	t2, _ := r.Next()
	if t2.Type != TokenString || t2.Str != "a" {
		t.Fatal("key")
	}
	t3, _ := r.Next()
	if t3.Type != TokenNumber || t3.Num != 1 {
		t.Fatal("num")
	}
}

func TestReader_Array(t *testing.T) {
	r := NewReader([]byte(`[1,"a",true,null]`))
	expected := []TokenType{TokenArrayStart, TokenNumber, TokenString, TokenBool, TokenNull, TokenArrayEnd}
	for _, e := range expected {
		tok, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		if tok.Type != e {
			t.Fatal("type", e)
		}
	}
}

func TestReader_EOF(t *testing.T) {
	r := NewReader([]byte(""))
	_, err := r.Next()
	if err == nil {
		t.Fatal("expected EOF")
	}
}

func TestReader_Bad(t *testing.T) {
	r := NewReader([]byte(`{xx}`))
	r.Next() // consume {
	if _, err := r.Next(); err == nil {
		t.Fatal("expected error")
	}
}
