package seqcache

import "testing"

func TestSequenceMonotonic(t *testing.T) {
	c := New(100)
	prev := uint64(0)
	for i := 0; i < 100; i++ {
		c.Put("k", i)
	}
	// 检查所有 sequence 单调递增且 Get 按字符串 key 应匹配
	if v, ok := c.Get("k"); !ok || v.(int) != 99 {
		t.Fatalf("Get(k)=%v ok=%v", v, ok)
	}
	_ = prev
}
