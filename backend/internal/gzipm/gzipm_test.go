package gzipm

import (
	"bytes"
	"io"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	payload := bytes.Repeat([]byte("hello "), 100)
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() >= len(payload) {
		t.Fatal("应小于原始字节数")
	}
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	if !bytes.Equal(got, payload) {
		t.Fatal("回环不一致")
	}
}

func TestAccepts(t *testing.T) {
	if !Accepts("gzip") {
		t.Fatal("gzip")
	}
	if !Accepts("gzip, deflate") {
		t.Fatal("gzip, deflate")
	}
	if Accepts("deflate") {
		t.Fatal("deflate")
	}
	if Accepts("") {
		t.Fatal("空")
	}
}

func TestWriteClosed(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Close()
	if _, err := w.Write([]byte("x")); err != ErrClosed {
		t.Fatal("应返回 ErrClosed")
	}
}

func TestReset(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	w := NewWriter(&buf1)
	w.Write([]byte("aaa"))
	w.Close()
	w.Reset(&buf2)
	w.Write([]byte("bbb"))
	w.Close()
	r, _ := NewReader(&buf2)
	got, _ := io.ReadAll(r)
	if string(got) != "bbb" {
		t.Fatal("reset 后输出错乱")
	}
}

func TestLevel(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriterLevel(&buf, gzip_BestCompression())
	w.Write([]byte("重复内容重复内容重复内容"))
	w.Close()
	if buf.Len() == 0 {
		t.Fatal("输出为空")
	}
}

func gzip_BestCompression() int { return 9 }

func TestEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	w.Close()
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(r)
	if len(got) != 0 {
		t.Fatal("空应为空")
	}
}
