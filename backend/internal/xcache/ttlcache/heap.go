// Package ttlcache 提供一个带过期时间(TTL)的内存缓存。
//
// 内部使用 map + 优先队列管理过期事件;
// 同一 key 的多次 Put 仅保留最新条目(旧的过期项会被同步移除,
// 避免堆无限增长)。
//
// 适用于读多写少、可容忍偶发 stale 读取的场景;
// 严格一致性请使用外部锁。
//
// 本包按职责拆分到多个文件:
//   - heap.go   item 与堆实现
//   - cache.go  Cache 主体 + Stats
package ttlcache

import "time"

// item 是堆中的一项过期事件。
//
// dead 字段用于延迟删除:同一 key 被多次 Put 时,旧条目会被标记,
// 等堆顶遇到时再丢弃,避免破坏 heap.Push 的唯一性假设。
type item struct {
	k    string    // 对应的 key
	t    time.Time // 过期时间
	dead bool      // 逻辑删除标记
}

// pq 是按过期时间升序排列的最小堆。
type pq []*item

func (p pq) Len() int           { return len(p) }
func (p pq) Less(i, j int) bool { return p[i].t.Before(p[j].t) }
func (p pq) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)        { *p = append(*p, x.(*item)) }
func (p *pq) Pop() any {
	o := *p
	n := len(o)
	x := o[n-1]
	*p = o[:n-1]
	return x
}
