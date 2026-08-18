// Sampling.
//
// A Sampler decides whether a new trace should be recorded. The SDK
// always records spans (so context propagation works), but when a
// sampler returns false the trace_id is still minted and propagated;
// the span can carry a sampled=false hint to downstream collectors
// that wish to drop the span cheaply.
//
// Built-in samplers:
//   - AlwaysOnSampler   (default)
//   - AlwaysOffSampler
//   - TraceIDRatioBased(ratio)  -- deterministic, hash-bucketed
//   - ParentBasedSampler   -- defer to the parents decision
package otlp

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// Sampler decides whether a trace should be sampled.
type Sampler interface {
	// ShouldSample returns whether this trace should be sampled.
	ShouldSample(ctx SampleContext) bool
	Description() string
}

// SampleContext is the input to ShouldSample.
type SampleContext struct {
	TraceID   string
	SpanID    string
	Name      string
	Parent    *SampleContext
	HasParent bool
}

// AlwaysOnSampler samples every trace.
type AlwaysOnSampler struct{}

func (AlwaysOnSampler) ShouldSample(_ SampleContext) bool { return true }
func (AlwaysOnSampler) Description() string               { return "AlwaysOnSampler" }

// AlwaysOffSampler drops every trace.
type AlwaysOffSampler struct{}

func (AlwaysOffSampler) ShouldSample(_ SampleContext) bool { return false }
func (AlwaysOffSampler) Description() string               { return "AlwaysOffSampler" }

// TraceIDRatioBased deterministically samples a fixed ratio of traces
// based on a hash of the trace_id. The same trace_id is always sampled
// the same way, which keeps trace trees intact.
type TraceIDRatioBased struct {
	ratio float64
}

// NewTraceIDRatioBased returns a sampler that samples roughly ratio of
// traces (ratio in [0, 1]).
func NewTraceIDRatioBased(ratio float64) *TraceIDRatioBased {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return &TraceIDRatioBased{ratio: ratio}
}

// ShouldSample hashes the trace_id and compares the first 8 bytes to
// math.MaxUint64 * ratio. This gives a uniform distribution that is
// stable across processes.
func (s *TraceIDRatioBased) ShouldSample(c SampleContext) bool {
	if s.ratio == 0 {
		return false
	}
	if s.ratio == 1 {
		return true
	}
	h := sha256.Sum256([]byte(c.TraceID))
	v := binary.BigEndian.Uint64(h[:8])
	threshold := uint64(float64(math.MaxUint64) * s.ratio)
	return v < threshold
}

func (s *TraceIDRatioBased) Description() string {
	return "TraceIDRatioBased" 
}

// ParentBasedSampler delegates the decision to the parent context. If
// no parent exists, it falls back to a delegate Sampler.
type ParentBasedSampler struct {
	root Sampler
}

// NewParentBasedSampler wraps a fallback sampler for root spans.
func NewParentBasedSampler(root Sampler) *ParentBasedSampler {
	return &ParentBasedSampler{root: root}
}

func (s *ParentBasedSampler) ShouldSample(c SampleContext) bool {
	if c.HasParent {
		return true
	}
	return s.root.ShouldSample(c)
}

func (s *ParentBasedSampler) Description() string {
	return "ParentBasedSampler{" + s.root.Description() + "}"
}

// CompositeAnd / CompositeOr let you combine samplers.
type compositeSampler struct {
	children []Sampler
	mode     string
}

func (s *compositeSampler) ShouldSample(c SampleContext) bool {
	for _, child := range s.children {
		sampled := child.ShouldSample(c)
		if s.mode == "and" && !sampled {
			return false
		}
		if s.mode == "or" && sampled {
			return true
		}
	}
	return s.mode == "and"
}

func (s *compositeSampler) Description() string {
	desc := s.children[0].Description()
	for _, c := range s.children[1:] {
		desc += " " + s.mode + " " + c.Description()
	}
	return "(" + desc + ")"
}

// CompositeAnd returns a sampler that samples when ALL children sample.
func CompositeAnd(children ...Sampler) Sampler {
	return &compositeSampler{children: children, mode: "and"}
}

// CompositeOr returns a sampler that samples when ANY child samples.
func CompositeOr(children ...Sampler) Sampler {
	return &compositeSampler{children: children, mode: "or"}
}

// avoid unused import warning when only always-on is used.
var _ = math.Floor
