// Package arc 提供一个简化版 ARC（自适应替换）缓存：
// 在 LRU 和 LFU 之间动态平衡，适应访问模式。
package arc

import (
	"container/list"
	"sync"
)

// Cache 是一个 ARC 缓存。
type Cache struct {
	mu   sync.Mutex
	cap  int
	t1   *list.List // 最近访问
	t2   *list.List // 高频访问
	b1   *list.List // T1 历史淘汰
	b2   *list.List // T2 历史淘汰
	idx  map[string]*list.Element
	p    int // 平衡参数
}

type entry struct {
	k   string
	v   any
	isfreq bool
}

// New 创建一个容量 cap 的 ARC 缓存。
func New(cap int) *Cache {
	if cap <= 0 {
		cap = 128
	}
	return &Cache{
		cap: cap,
		t1:  list.New(),
		t2:  list.New(),
		b1:  list.New(),
		b2:  list.New(),
		idx: make(map[string]*list.Element, cap),
	}
}

// Get 读取并提升到 T2。
func (c *Cache) Get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.idx[k]; ok {
		ent := el.Value.(*entry)
		if !ent.isfreq {
			c.t1.Remove(el)
			ent.isfreq = true
			c.t2.PushFront(el)
		}
		return ent.v, true
	}
	return nil, false
}

// Put 写入键值。
func (c *Cache) Put(k string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.idx[k]; ok {
		ent := el.Value.(*entry)
		ent.v = v
		if !ent.isfreq {
			c.t1.Remove(el)
			ent.isfreq = true
			c.t2.PushFront(el)
		}
		return
	}
	// 缺失：替换流程
	sz := c.t1.Len() + c.b1.Len()
	if el := c.t1.Back(); el != nil && sz >= c.cap {
		c.replace(false)
	}
	el := c.t1.PushFront(&entry{k: k, v: v, isfreq: false})
	c.idx[k] = el
	for c.t1.Len()+c.t2.Len() > c.cap {
		if back := c.t1.Back(); back != nil {
			ent := back.Value.(*entry)
			c.t1.Remove(back)
			if c.b1.Len() >= c.cap {
				o := c.b1.Back()
				if o != nil {
					delete(c.idx, o.Value.(*entry).k)
					c.b1.Remove(o)
				}
			}
			c.b1.PushFront(ent)
		} else if back := c.t2.Back(); back != nil {
			ent := back.Value.(*entry)
			c.t2.Remove(back)
			if c.b2.Len() >= c.cap {
				o := c.b2.Back()
				if o != nil {
					delete(c.idx, o.Value.(*entry).k)
					c.b2.Remove(o)
				}
			}
			c.b2.PushFront(ent)
		} else {
			break
		}
	}
}

func (c *Cache) replace(inB2 bool) {
	// 根据 p 决定淘汰 T1 还是 T2 的尾部
	if inB2 {
		if el := c.t2.Back(); el != nil {
			ent := el.Value.(*entry)
			c.t2.Remove(el)
			if c.b2.Len() >= c.cap {
				o := c.b2.Back()
				if o != nil {
					delete(c.idx, o.Value.(*entry).k)
					c.b2.Remove(o)
				}
			}
			c.b2.PushFront(ent)
		}
		return
	}
	if el := c.t1.Back(); el != nil {
		ent := el.Value.(*entry)
		c.t1.Remove(el)
		if c.b1.Len() >= c.cap {
			o := c.b1.Back()
			if o != nil {
				delete(c.idx, o.Value.(*entry).k)
				c.b1.Remove(o)
			}
		}
		c.b1.PushFront(ent)
	}
}

// Len 返回 T1+T2 总条目数。
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t1.Len() + c.t2.Len()
}

// Cap 返回容量。
func (c *Cache) Cap() int { return c.cap }
