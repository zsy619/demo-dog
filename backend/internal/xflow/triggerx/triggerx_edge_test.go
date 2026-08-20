package triggerx

import (
	"sync/atomic"
	"testing"
)

func TestCancelReordered(t *testing.T) {
	tr := New[int]()
	var firstCalled, secondCalled, thirdCalled atomic.Int32
	cancelFirst := tr.Add(func(int) { firstCalled.Add(1) })
	cancelSecond := tr.Add(func(int) { secondCalled.Add(1) })
	tr.Add(func(int) { thirdCalled.Add(1) })
	cancelFirst()  // handlers = [second, third]
	cancelSecond() // idx=1, but handlers[1]=third now! Remove wrong
	tr.Fire(1)
	if secondCalled.Load() != 0 {
		t.Fatalf("second 应被取消，实际 %d 次", secondCalled.Load())
	}
	if thirdCalled.Load() != 1 {
		t.Fatalf("third 应被保留，实际 %d 次", thirdCalled.Load())
	}
}
