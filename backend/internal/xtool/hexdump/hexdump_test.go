package hexdump

import (
	"bytes"
	"strings"
	"testing"
)

func TestDump(t *testing.T) {
	var buf bytes.Buffer
	Dump(&buf, []byte("hello world\n"))
	out := buf.String()
	if !strings.Contains(out, "00000000") {
		t.Fatal("addr")
	}
	if !strings.Contains(out, "hello world") {
		t.Fatal("ascii")
	}
}

func TestToString(t *testing.T) {
	s := ToString([]byte{0x00, 0x01, 0x02, 0x03})
	if !strings.Contains(s, "00 01 02 03") {
		t.Fatal("str")
	}
}

func TestDump_Long(t *testing.T) {
	data := make([]byte, 100)
	for i := range data {
		data[i] = byte(i)
	}
	var buf bytes.Buffer
	Dump(&buf, data)
	if !strings.Contains(buf.String(), "00000010") {
		t.Fatal("multi line")
	}
}

func TestDump_Empty(t *testing.T) {
	var buf bytes.Buffer
	Dump(&buf, nil)
	if buf.Len() != 0 {
		t.Fatal("empty")
	}
}
