package api

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// AuditEvent 是审计日志中的一行。我们刻意保持
// 模式极简，以便写入器在高负载下零分配。
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	KeyLabel  string    `json:"key_label,omitempty"`
	Role      string    `json:"role,omitempty"`
	Tenant    string    `json:"tenant,omitempty"`
	Status    int       `json:"status"`
	BytesIn   int64     `json:"bytes_in"`
	BytesOut  int64     `json:"bytes_out"`
	RemoteIP  string    `json:"remote_ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// AuditLog 是最近写操作的有界环形缓冲区。
// Recent(n) 返回最近的 n 条事件；Filter() 返回
// 与给定查询匹配的事件。缓冲区由 `cap` 限制
//（默认 10 000）。单独的清扫 goroutine 会在
// 配置了留存 TTL 时丢弃超过 TTL 的事件。
type AuditLog struct {
	mu            sync.RWMutex
	cap           int
	events        []AuditEvent
	writeCt       uint64
	retentionTTL  time.Duration
	retentionStop chan struct{}
}

// NewAuditLog 返回容量为 `cap` 的缓冲区。
// 默认容量（cap <= 0 时）为 10 000 条。
func NewAuditLog(cap int) *AuditLog {
	if cap <= 0 {
		cap = 10_000
	}
	return &AuditLog{cap: cap}
}

// Append 写入一条事件。我们每次调用获取一次写锁；
// 缓冲区的拷贝是 O(1) 的，因为我们最多
// 将切片扩展到 `cap`，然后开始覆盖。
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

// Recent 返回最多 n 条最近的事件，按从旧到新排序。
// 当 n <= 0 时返回整个缓冲区。
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

// EncodeJSON 将缓冲区编码为 JSON 数组。
func (a *AuditLog) EncodeJSON() ([]byte, error) {
	return json.MarshalIndent(a.Recent(0), "", "  ")
}

// SetRetentionTTL 配置一个自动清扫，丢弃
// 早于给定时间的事件。传入 0 可禁用。
// 清扫在后台 goroutine 中每分钟运行一次，不会阻塞 Append。
func (a *AuditLog) SetRetentionTTL(ttl time.Duration) {
	a.mu.Lock()
	a.retentionTTL = ttl
	stop := a.retentionStop
	a.mu.Unlock()
	if stop != nil {
		return // already running
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

// Close 停止留存清扫。幂等。
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

// Filter 返回最多 n 条匹配所有给定过滤器的事件。
// 所有非空过滤器都必须匹配（逻辑 AND）。
// 传入 0 表示返回所有匹配项。
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

// AuditFilter 是 Filter 的查询 DSL。空字段表示 "任意"。
// 
type AuditFilter struct {
	Method    string
	Path      string
	KeyLabel  string
	Tenant    string
	StatusMin int
	StatusMax int
	Since     time.Time
	Until     time.Time
}

func (f AuditFilter) matches(e AuditEvent) bool {
	if f.Method != "" && e.Method != f.Method {
		return false
	}
	if f.Path != "" && !strings.Contains(e.Path, f.Path) {
		return false
	}
	if f.KeyLabel != "" && e.KeyLabel != f.KeyLabel {
		return false
	}
	if f.Tenant != "" && e.Tenant != f.Tenant {
		return false
	}
	if f.StatusMin > 0 && e.Status < f.StatusMin {
		return false
	}
	if f.StatusMax > 0 && e.Status > f.StatusMax {
		return false
	}
	if !f.Since.IsZero() && e.Timestamp.Before(f.Since) {
		return false
	}
	if !f.Until.IsZero() && e.Timestamp.After(f.Until) {
		return false
	}
	return true
}
