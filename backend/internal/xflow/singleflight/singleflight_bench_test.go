package singleflight

import "testing"

func BenchmarkDo(b *testing.B) {
	g := New[string, int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Do("k", func() (int, error) { return 1, nil })
	}
}

func BenchmarkDoConcurrent(b *testing.B) {
	g := New[string, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.Do("k", func() (int, error) { return 1, nil })
		}
	})
}
