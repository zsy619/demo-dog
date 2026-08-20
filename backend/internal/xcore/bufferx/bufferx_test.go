package bufferx

import (
	"bytes"
	"io"
	"testing"
)

func TestWriteRead(t *testing.T) {
	b := New(16)
	if _, err := b.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	out, err := b.Read(3)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hel" {
		t.Fatal("read", out)
	}
	if b.Len() != 2 {
		t.Fatal("len", b.Len())
	}
}

func TestBytes(t *testing.T) {
	b := New(8)
	b.Write([]byte("abc"))
	if string(b.Bytes()) != "abc" {
		t.Fatal("bytes")
	}
	// 修改副本不应影响 buffer
	out := b.Bytes()
	out[0] = 'X'
	if b.Bytes()[0] != 'a' {
		t.Fatal("alias")
	}
}

func TestReset(t *testing.T) {
	b := New(8)
	b.Write([]byte("x"))
	b.Reset()
	if b.Len() != 0 {
		t.Fatal("reset")
	}
}

func TestWriteByte(t *testing.T) {
	b := New(8)
	if err := b.WriteByte('a'); err != nil {
		t.Fatal(err)
	}
	if b.Len() != 1 {
		t.Fatal("byte")
	}
}

func TestWriteString(t *testing.T) {
	b := New(8)
	n, err := b.WriteString("hi")
	if err != nil || n != 2 {
		t.Fatal("str", n, err)
	}
}

func TestReadAll(t *testing.T) {
	b := New(8)
	b.Write([]byte("xyz"))
	all := b.ReadAll()
	if string(all) != "xyz" {
		t.Fatal("all")
	}
	if b.Len() != 0 {
		t.Fatal("not consumed")
	}
}

func TestBounded(t *testing.T) {
	b := NewBounded(0, 4)
	if _, err := b.Write([]byte("abcd")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("e")); err != ErrTooLarge {
		t.Fatal("应超限", err)
	}
	if b.Len() != 4 {
		t.Fatal("写入超限应被拒")
	}
}

func TestNegativeRead(t *testing.T) {
	b := New(8)
	if _, err := b.Read(-1); err != ErrNegativeRead {
		t.Fatal("应返 ErrNegativeRead")
	}
}

func TestIOWriterInterface(t *testing.T) {
	var w io.Writer = New(8)
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
}

func TestIOWriterString(t *testing.T) {
	var w io.StringWriter = New(8)
	if _, err := w.WriteString("abc"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrent(t *testing.T) {
	b := New(1024)
	done := make(chan bool, 10)
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				b.Write([]byte{byte(j)})
			}
			done <- true
		}()
	}
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				b.Read(10)
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestZeroAfterRead(t *testing.T) {
	b := New(8)
	b.Write([]byte("secret"))
	out, _ := b.Read(6)
	if !bytes.Equal(out, []byte("secret")) {
		t.Fatal("output")
	}
	// 内部数组已被零化
	for _, v := range b.Bytes() {
		if v != 0 {
			t.Fatalf("未零化: %v", b.Bytes())
		}
	}
}
