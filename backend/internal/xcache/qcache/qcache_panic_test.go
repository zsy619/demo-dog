package qcache

import "testing"

func TestLoaderPanic(t *testing.T) {
	c := New(func(k string) (int, error) {
		panic("boom")
	})
	_, err := c.Get("k")
	if err == nil {
		t.Fatal("应返回错误")
	}
	// 第二次调用不应挂死
	_, err2 := c.Get("k")
	if err2 == nil {
		t.Fatal("第二次也应返回错误")
	}
}
