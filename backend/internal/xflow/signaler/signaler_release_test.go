package signaler

import "testing"

func TestReleaseWithoutAcquire(t *testing.T) {
	s := New(2)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Release 在空信号量上 panic: %v", r)
		}
	}()
	s.Release() // 应不 panic
}
