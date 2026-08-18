package api

import (
	"encoding/json"
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

// AuditLog is a bounded ring buffer of recent write operations. We
// keep the most recent `cap` events and drop the oldest; a real
// deployment would forward to Loki/Splunk via an external sink, but
// the demo gets everything it needs from a single GET endpoint.
type AuditLog struct {
	mu      sync.RWMutex
	cap     int
	events  []AuditEvent
	writeCt uint64
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
		// Ring-style overwrite. The first slot is the oldest; new
		// event lands there and the cursor advances.
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
	// If the buffer has wrapped, slice from the write cursor
	// (oldest) and stitch through to the end.
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
		"buffered": len(a.events),
		"capacity": a.cap,
		"total":    a.writeCt,
	}
}

// EncodeJSON returns the buffer as a JSON array. Used by the audit
// endpoint handler.
func (a *AuditLog) EncodeJSON() ([]byte, error) {
	return json.MarshalIndent(a.Recent(0), "", "  ")
}
