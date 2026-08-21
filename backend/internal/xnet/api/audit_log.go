package api

// audit_log.go:AuditLog 主体(环形缓冲 + retention 扫描)。
//
// Recent(n) 返回最近的 n 条事件;Filter() 按 AuditFilter 过滤。
// 缓冲上限为 cap(默认 10 000);配置了 retention TTL 时,
// 一个独立的 sweeper goroutine 会丢弃早于该 TTL 的事件。

import (
	"encoding/json"
	"sync"
	"time"
)

// AuditLog 是有界的环形缓冲,保存最近写操作。
type AuditLog struct {
	mu            sync.RWMutex // 保护 events / retention
	cap           int          // 容量上限
	events        []AuditEvent // 事件缓冲
	writeCt       uint64       // 累计写入数(用于环形索引)
	retentionTTL  time.Duration // 留存 TTL(0 表示禁用)
	retentionStop chan struct{} // 扫描 goroutine 停止信号
}

// NewAuditLog 返回容量为 cap 的缓冲;cap <= 0 默认 10 000。
func NewAuditLog(cap int) *AuditLog {
	if cap <= 0 {
		cap = 10_000
	}
	return &AuditLog{cap: cap}
}

// Append 存储一条事件。
//
// 每调用取一次写锁;缓冲复制 O(1),因为切片最多长到 cap,然后开始覆盖。
func (a *AuditLog) Append(ev AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.events) < a.cap {
		a.events = append(a.events, ev)
	} else {
		idx := int(a.writeCt) % a.cap
		a.events[idx] = ev
	}
	a.writeCt++
}

// Recent 返回最多 n 条最近的事件,按时间从旧到新排序。
//
// n <= 0 返回整个缓冲。
func (a *AuditLog) Recent(n int) []AuditEvent {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.events) == 0 {
		return nil
	}
	var out []AuditEvent
	if a.writeCt <= uint64(a.cap) {
		out = make([]AuditEvent, len(a.events))
		copy(out, a.events)
	} else {
		start := int(a.writeCt) % a.cap
		out = make([]AuditEvent, a.cap)
		copy(out, a.events[start:])
		copy(out[a.cap-start:], a.events[:start])
	}
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

// Stats 返回一个适合 /api/audit/stats 的小摘要。
func (a *AuditLog) Stats() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return map[string]any{
		"buffered":  len(a.events),
		"capacity":  a.cap,
		"total":     a.writeCt,
		"retention": a.retentionTTL.String(),
	}
}

// EncodeJSON 把缓冲区编码为 JSON 数组。
func (a *AuditLog) EncodeJSON() ([]byte, error) {
	return json.MarshalIndent(a.Recent(0), "", "  ")
}

// SetRetentionTTL 配置自动扫描,丢弃早于指定时长的事件。
//
// 传入 0 表示禁用。扫描每分钟一次,后台 goroutine 运行,不阻塞 Append。
func (a *AuditLog) SetRetentionTTL(ttl time.Duration) {
	a.mu.Lock()
	a.retentionTTL = ttl
	stop := a.retentionStop
	a.mu.Unlock()
	if stop != nil {
		return // 已在运行
	}
	if ttl <= 0 {
		return
	}
	stopChan := make(chan struct{})
	a.mu.Lock()
	a.retentionStop = stopChan
	a.mu.Unlock()
	go a.sweep(stopChan)
}

// Close 停止留存清扫,幂等。
func (a *AuditLog) Close() {
	a.mu.Lock()
	stop := a.retentionStop
	a.retentionStop = nil
	a.mu.Unlock()
	if stop == nil {
		return
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
}

// sweep 是 retention TTL 的后台扫描循环。
func (a *AuditLog) sweep(stop chan struct{}) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			a.mu.Lock()
			ttl := a.retentionTTL
			if ttl > 0 {
				cutoff := time.Now().Add(-ttl)
				// 遍历活跃缓冲区并丢弃前部的旧事件。
				drop := 0
				for _, e := range a.events {
					if e.Timestamp.Before(cutoff) {
						drop++
					} else {
						break
					}
				}
				if drop > 0 {
					a.events = a.events[drop:]
				}
			}
			a.mu.Unlock()
		}
	}
}

// Filter 返回最多 n 条匹配所有提供过滤器的事件。
//
// 所有非空过滤器都必须匹配(逻辑与);n == 0 时返回所有匹配项。
func (a *AuditLog) Filter(n int, f AuditFilter) []AuditEvent {
	recent := a.Recent(0)
	out := make([]AuditEvent, 0, len(recent))
	for _, e := range recent {
		if f.matches(e) {
			out = append(out, e)
		}
	}
	if n > 0 && len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}
