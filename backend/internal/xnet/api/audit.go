package api

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
)

// AuditEvent is one line in the audit log. We deliberately keep
// the schema tiny so the writer is allocation-free under load.
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

// AuditLog is a bounded ring buffer of recent write operations.
// Recent(n) returns the most recent `n` events; Filter() returns
// events matching the provided query. The buffer is bounded by `cap`
// (10 000 by default). A separate sweeper goroutine drops events
// older than the retention TTL when one is configured.
type AuditLog struct {
	mu            sync.RWMutex
	cap           int
	events        []AuditEvent
	writeCt       uint64
	retentionTTL  time.Duration
	retentionStop chan struct{}
}

// NewAuditLog returns a buffer sized to hold `cap` events. Default
// capacity (when cap <= 0) is 10 000 entries.
func NewAuditLog(cap int) *AuditLog {
	if cap <= 0 {
		cap = 10_000
	}
	return &AuditLog{cap: cap}
}

// Append stores one event. We acquire the write lock once per call;
// the buffer copy is O(1) because we only ever grow the slice up to
// `cap` and then start overwriting.
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

// Recent returns up to `n` of the most-recent events, oldest first.
// When n <= 0 the entire buffer is returned.
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

// Stats returns a small summary suitable for /api/audit/stats.
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

// EncodeJSON returns the buffer as a JSON array.
func (a *AuditLog) EncodeJSON() ([]byte, error) {
	return json.MarshalIndent(a.Recent(0), "", "  ")
}

// SetRetentionTTL configures an automatic sweep that drops events
// older than the given duration. Pass 0 to disable. The sweep runs
// in a background goroutine every minute; it does NOT block Append.
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

// Close stops the retention sweeper. Idempotent.
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
				// Walk the live buffer and drop leading old events.
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

// Filter returns up to `n` events matching all the provided filters.
// All non-empty filters must match (logical AND). Pass 0 for n to
// return every match.
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

// AuditFilter is the query DSL for Filter. Empty fields are
// "any".
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
