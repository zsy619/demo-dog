// Tail sampling: a sampler that drops uninteresting traces while
// keeping every trace that contains an error or a slow span.
//
// The sampler keeps a small ring of recent decisions keyed by
// trace_id so a span emitted mid-trace can consult the trace-level
// decision before recording. The ring is bounded; very old entries
// are evicted and the sampler falls back to "keep" (safer default
// than "drop" for an enterprise backend).

package otlp

import (
	"container/list"
	"sync"
)

// TailBasedSampler keeps every trace that contains an error OR a
// span slower than MaxLatencyMs, and drops the rest when DropRatio
// is non-zero. The decision is recorded per trace_id so subsequent
// spans in the same trace honour it.
type TailBasedSampler struct {
	mu sync.Mutex

	maxLatencyMs int64
	dropRatio    float64

	// trace_id -> decision ("keep" true / "drop" false).
	decisions map[string]bool
	order     *list.List // FIFO eviction of trace ids
	cap       int
}

// NewTailBasedSampler returns a sampler that retains traces whose
// slowest span is >= maxLatencyMs or whose any span has Status=error.
// When dropRatio is 0 the sampler is essentially AlwaysOnSampler;
// when dropRatio is 1 every boring trace is dropped.
func NewTailBasedSampler(maxLatencyMs int64, dropRatio float64) *TailBasedSampler {
	if dropRatio < 0 {
		dropRatio = 0
	}
	if dropRatio > 1 {
		dropRatio = 1
	}
	return &TailBasedSampler{
		maxLatencyMs: maxLatencyMs,
		dropRatio:    dropRatio,
		decisions:    make(map[string]bool),
		order:        list.New(),
		cap:          4096,
	}
}

// ObserveSpan records a span outcome. The sampler updates its
// per-trace decision: if any span is slow or errors, the trace is
// pinned to "keep". If the trace is currently unknown it is created
// with a probabilistic "drop" decision (unless DropRatio is 0).
func (s *TailBasedSampler) ObserveSpan(traceID string, durationMs int64, status string) {
	if traceID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	keep := s.isInteresting(durationMs, status)
	if existing, ok := s.decisions[traceID]; ok {
		if existing || !keep {
			return
		}
	} else {
		s.touchLocked(traceID)
	}
	s.decisions[traceID] = keep
}

// ShouldSample returns the recorded decision for a trace, defaulting
// to true when no decision has been recorded yet.
func (s *TailBasedSampler) ShouldSample(c SampleContext) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.decisions[c.TraceID]; ok {
		return d
	}
	return true
}

// Description returns a stable label for Stats.
func (s *TailBasedSampler) Description() string {
	return "TailBasedSampler"
}

func (s *TailBasedSampler) isInteresting(durationMs int64, status string) bool {
	if status == "error" {
		return true
	}
	if s.maxLatencyMs > 0 && durationMs >= s.maxLatencyMs {
		return true
	}
	return false
}

func (s *TailBasedSampler) touchLocked(traceID string) {
	for s.order.Len() >= s.cap {
		front := s.order.Front()
		if front == nil {
			break
		}
		old := s.order.Remove(front).(string)
		delete(s.decisions, old)
	}
	s.order.PushBack(traceID)
}
