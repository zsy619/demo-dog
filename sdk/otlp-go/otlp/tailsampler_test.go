package otlp

import "testing"

func TestTailBased_KeepErrors(t *testing.T) {
	s := NewTailBasedSampler(100, 0.5)
	s.ObserveSpan("t-1", 1, "error")
	if !s.ShouldSample(SampleContext{TraceID: "t-1"}) {
		t.Errorf("error trace should be kept")
	}
}

func TestTailBased_KeepSlow(t *testing.T) {
	s := NewTailBasedSampler(100, 1.0)
	s.ObserveSpan("t-1", 150, "ok")
	if !s.ShouldSample(SampleContext{TraceID: "t-1"}) {
		t.Errorf("slow trace should be kept at threshold")
	}
}

func TestTailBased_UnknownDefault(t *testing.T) {
	s := NewTailBasedSampler(100, 1.0)
	if !s.ShouldSample(SampleContext{TraceID: "unknown"}) {
		t.Errorf("unknown trace should default to keep")
	}
}

func TestTailBased_BoringDropped(t *testing.T) {
	s := NewTailBasedSampler(100, 1.0)
	s.ObserveSpan("t-boring", 1, "ok")
	// Boring trace with no error and below threshold. Should be
	// recorded as "drop" in the decision map.
	if s.ShouldSample(SampleContext{TraceID: "t-boring"}) {
		t.Errorf("boring trace should be dropped")
	}
}

func TestTailBased_PinnedKeepNeverDowngrades(t *testing.T) {
	s := NewTailBasedSampler(0, 1.0)
	s.ObserveSpan("t", 0, "error")
	// Late observation is "ok" but the trace is already kept.
	s.ObserveSpan("t", 0, "ok")
	if !s.ShouldSample(SampleContext{TraceID: "t"}) {
		t.Errorf("pinned keep should never downgrade")
	}
}
