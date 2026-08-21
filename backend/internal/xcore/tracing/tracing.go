// Package tracing 调用链路追踪：Span / Trace ID 上下文传播。
package tracing

// Distributed tracing primitives.
//
// Until now the 副本 side had a thin W3C trace context
// parser (Round 40). Round 54 adds the in-memory Span /
// TraceStore that every component 写入 into, plus the
// sampling decision logic that decides whether to keep or
// drop a span.
//
// The store is ring-buffered (bounded memory); old spans are
// evicted FIFO. Each span carries the W3C trace context
// fields plus the standard OpenTelemetry span 属性.
//
// Stdlib-only: no third-party deps. The HTTP / OTLP export
// is wired in cmd/dog-collector (out of scope for this
// package).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// SpanKind identifies the role of a span.
type SpanKind string

const (
	KindServer SpanKind = "server"
	KindClient SpanKind = "client"
	KindInternal SpanKind = "internal"
	KindProducer SpanKind = "producer"
	KindConsumer SpanKind = "consumer"
)

// Span is one unit of work.
type Span struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	Kind       SpanKind          `json:"kind"`
	Start      time.Time         `json:"start"`
	End        time.Time         `json:"end"`
	Status     string            `json:"status"` // ok, error, unset
	StatusMsg  string            `json:"status_msg,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Tenant     string            `json:"tenant,omitempty"`
}

// Duration 返回 End - Start, or 0 if the span is open.
func (s *Span) Duration() time.Duration {
	if s.End.IsZero() {
		return 0
	}
	return s.End.Sub(s.Start)
}

// TraceStore keeps the most recent spans.
type TraceStore struct {
	mu       sync.RWMutex
	cap      int
	spans    []*Span
	index    map[string]int // span_id -> index in spans
	head     int
	count    atomic.Int64 // total spans ever seen (for metrics)
	dropped  atomic.Int64 // total spans dropped due to cap
}

// NewTraceStore 返回 a ring-buffered store.
func NewTraceStore(capacity int) *TraceStore {
	if capacity <= 0 {
		capacity = 1024
	}
	return &TraceStore{
		cap:   capacity,
		spans:  make([]*Span, capacity),
		index:  make(map[string]int, capacity),
	}
}

// Add 记录 a finished span. If the store is full, the
// oldest span is evicted.
func (s *TraceStore) Add(span *Span) {
	if span == nil || span.SpanID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx, ok := s.index[span.SpanID]; ok {
		s.spans[idx] = span
		s.count.Add(1)
		return
	}
	if len(s.index) < s.cap {
		for i, slot := range s.spans {
			if slot == nil {
				s.spans[i] = span
				s.index[span.SpanID] = i
				s.count.Add(1)
				return
			}
		}
	}
	// Ring eviction.
	evict := s.spans[s.head]
	if evict != nil {
		delete(s.index, evict.SpanID)
		s.dropped.Add(1)
	}
	s.spans[s.head] = span
	s.index[span.SpanID] = s.head
	s.head = (s.head + 1) % s.cap
	s.count.Add(1)
}

// Get 返回 a span by id.
func (s *TraceStore) Get(spanID string) (*Span, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.index[spanID]
	if !ok {
		return nil, false
	}
	return s.spans[i], true
}

// ByTrace 返回 all spans for a trace, sorted by Start.
func (s *TraceStore) ByTrace(traceID string) []*Span {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Span, 0, 8)
	for _, sp := range s.spans {
		if sp != nil && sp.TraceID == traceID {
			out = append(out, sp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// List 返回 all spans (快照, may be up to capacity).
func (s *TraceStore) List() []*Span {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Span, 0, len(s.index))
	for _, sp := range s.spans {
		if sp != nil {
			out = append(out, sp)
		}
	}
	return out
}

// Count 返回 running totals.
func (s *TraceStore) Count() (total, dropped int64) {
	return s.count.Load(), s.dropped.Load()
}

// Stats is the JSON-stable view.
type Stats struct {
	Capacity int   `json:"capacity"`
	Size     int   `json:"size"`
	Total    int64 `json:"total"`
	Dropped  int64 `json:"dropped"`
}

// Stats 返回 trace store counters.
func (s *TraceStore) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Stats{Capacity: s.cap, Size: len(s.index), Total: s.count.Load(), Dropped: s.dropped.Load()}
}

// SpanBuilder is a fluent helper for building spans without
// worrying about ID generation.
type SpanBuilder struct {
	store *TraceStore
	span  *Span
}

// StartSpan opens 新的 span on the store. trace/parent IDs
// are auto-generated if empty. 返回 a builder so callers
// can chain Set / End / Save.
func StartSpan(store *TraceStore, name string, kind SpanKind) *SpanBuilder {
	now := time.Now()
	tid := RandID(16)
	sid := RandID(8)
	return &SpanBuilder{
		store: store,
		span: &Span{
			TraceID:    tid,
			SpanID:     sid,
			Name:       name,
			Kind:       kind,
			Start:      now,
			Status:     "unset",
			Attributes: make(map[string]string),
		},
	}
}

// WithTrace attaches an existing trace ID.
func (b *SpanBuilder) WithTrace(traceID string) *SpanBuilder {
	if traceID != "" {
		b.span.TraceID = traceID
	}
	return b
}

// WithParent attaches an existing parent span ID.
func (b *SpanBuilder) WithParent(parentID string) *SpanBuilder {
	if parentID != "" {
		b.span.ParentID = parentID
	}
	return b
}

// WithTenant attaches the 租户 label.
func (b *SpanBuilder) WithTenant(tenant string) *SpanBuilder {
	b.span.Tenant = tenant
	return b
}

// Set adds an attribute.
func (b *SpanBuilder) Set(k, v string) *SpanBuilder {
	if b.span.Attributes == nil {
		b.span.Attributes = make(map[string]string)
	}
	b.span.Attributes[k] = v
	return b
}

// EndOK marks the span successful and saves it.
func (b *SpanBuilder) EndOK() *Span {
	b.span.End = time.Now()
	b.span.Status = "ok"
	b.save()
	return b.span
}

// EndError marks the span errored with a message.
func (b *SpanBuilder) EndError(msg string) *Span {
	b.span.End = time.Now()
	b.span.Status = "error"
	b.span.StatusMsg = msg
	b.save()
	return b.span
}

func (b *SpanBuilder) save() {
	if b.store != nil {
		b.store.Add(b.span)
	}
}

// Sampler decides whether to keep a span.
type Sampler struct {
	mu        sync.Mutex
	rate      float64 // 0..1
	salt      string
	count     atomic.Int64
	kept      atomic.Int64
	rng       func() float64
}

// NewSampler 返回 a probabilistic sampler.
func NewSampler(rate float64) *Sampler {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &Sampler{
		rate: rate,
		salt: RandID(8),
	}
}

// ShouldSample 返回 true when the span should be kept.
// The decision is deterministic per trace ID (so all spans
// in a trace agree).
func (s *Sampler) ShouldSample(traceID string) bool {
	s.count.Add(1)
	if s.rate >= 1 {
		s.kept.Add(1)
		return true
	}
	if s.rate <= 0 {
		return false
	}
	h := hashFraction(traceID + s.salt)
	if h < s.rate {
		s.kept.Add(1)
		return true
	}
	return false
}

// Rate 返回 sampler rate.
func (s *Sampler) Rate() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rate
}

// SetRate updates the sampler rate.
func (s *Sampler) SetRate(rate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rate = clamp(rate, 0, 1)
	s.rate = rate
}

// Stats is the JSON-stable view.
type SamplerStats struct {
	Rate     float64 `json:"rate"`
	Count    int64  `json:"count"`
	Kept     int64  `json:"kept"`
	DropRate float64 `json:"drop_rate"`
}

// Stats 返回 sampler counters.
func (s *Sampler) Stats() SamplerStats {
	s.mu.Lock()
	rate := s.rate
	s.mu.Unlock()
	count := s.count.Load()
	kept := s.kept.Load()
	drop := 0.0
	if count > 0 {
		drop = 1 - float64(kept)/float64(count)
	}
	return SamplerStats{Rate: rate, Count: count, Kept: kept, DropRate: drop}
}

// RandID 返回 a hex string of length n/2 bytes.
func RandID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// MarshalJSON is a custom encoder so empty attribute maps do
// not pollute the output.
func (s *Span) MarshalJSONAlias() ([]byte, error) {
	type alias Span
	return json.Marshal((*alias)(s))
}

// Validate runs basic integrity 检查 on a span.
func (s *Span) Validate() error {
	if len(s.TraceID) != 32 {
		return errors.New("trace_id must be 32 hex chars (16 bytes)")
	}
	if len(s.SpanID) != 16 {
		return errors.New("span_id must be 16 hex chars (8 bytes)")
	}
	if s.ParentID != "" && len(s.ParentID) != 16 {
		return errors.New("parent_id must be 16 hex chars when present")
	}
	return nil
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// hashFraction 返回 a deterministic value in [0, 1) for
// the input. Uses FNV-1a; the stdlib has no good 64-bit
// hash-to-fraction helper.
func hashFraction(s string) float64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return float64(h>>11) / float64(1<<53)
}
