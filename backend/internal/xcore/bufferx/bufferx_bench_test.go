package bufferx

import (
	"testing"
)

func BenchmarkBufferWrite(b *testing.B) {
	buf := New(0)
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Write(data)
		buf.Reset()
	}
}

func BenchmarkBufferRead(b *testing.B) {
	buf := New(1024)
	buf.Write(make([]byte, 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Read(512)
		if buf.Len() == 0 {
			buf.Write(make([]byte, 1024))
		}
	}
}

func BenchmarkBufferBytes(b *testing.B) {
	buf := New(1024)
	buf.Write(make([]byte, 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.Bytes()
	}
}

func BenchmarkBufferConcurrent(b *testing.B) {
	buf := New(4096)
	data := []byte("x")
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf.Write(data)
		}
	})
}
