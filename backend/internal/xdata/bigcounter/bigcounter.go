// Package bigcounter 提供 big.Int 计数的并发计数器。
package bigcounter

import (
	"math/big"
	"sync"
)

// Counter 是一个使用 big.Int 的原子计数器。
type Counter struct {
	mu sync.Mutex
	v  big.Int
}

// New 创建一个从 start 开始的大整数计数器。
func New(start int64) *Counter {
	c := &Counter{}
	c.v.SetInt64(start)
	return c
}

// Add 增加 delta。
func (c *Counter) Add(delta int64) {
	c.mu.Lock()
	c.v.Add(&c.v, big.NewInt(delta))
	c.mu.Unlock()
}

// Value 返回当前值（拷贝）。
func (c *Counter) Value() big.Int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return *(&big.Int{}).Set(&c.v)
}

// String 返回当前值的十进制表示。
func (c *Counter) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.v.String()
}
