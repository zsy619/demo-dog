package otlp

import (
	"fmt"
	"strings"
	"testing"
)

func TestAlwaysOn(t *testing.T) {
	s := AlwaysOnSampler{}
	if !s.ShouldSample(SampleContext{TraceID: "abc"}) {
		t.Fail()
	}
	if !strings.Contains(s.Description(), "AlwaysOn") {
		t.Errorf("desc: %s", s.Description())
	}
}

func TestAlwaysOff(t *testing.T) {
	s := AlwaysOffSampler{}
	if s.ShouldSample(SampleContext{TraceID: "abc"}) {
		t.Fail()
	}
}

func TestTraceIDRatioStable(t *testing.T) {
	s := NewTraceIDRatioBased(0.5)
	tid := "00000000000000000000000000000001"
	a := s.ShouldSample(SampleContext{TraceID: tid})
	for i := 0; i < 100; i++ {
		b := s.ShouldSample(SampleContext{TraceID: tid})
		if a != b {
			t.Fatalf("not stable: %v vs %v", a, b)
		}
	}
}

func TestTraceIDRatioDistribution(t *testing.T) {
	// Over 10000 unique trace IDs at ratio 0.1, sampled count should be
	// near 1000 (we accept 500..1500 to absorb hash skew).
	s := NewTraceIDRatioBased(0.1)
	var sampled int
	for i := 0; i < 10000; i++ {
		// Build a unique 32-char hex-ish ID from i.
		tid := fmt.Sprintf("%032x", i)
		if s.ShouldSample(SampleContext{TraceID: tid}) {
			sampled++
		}
	}
	if sampled < 500 || sampled > 1500 {
		t.Errorf("sampled=%d, expected ~1000", sampled)
	}
}

func TestCompositeAndOr(t *testing.T) {
	on := AlwaysOnSampler{}
	off := AlwaysOffSampler{}
	if CompositeAnd(on, off).ShouldSample(SampleContext{}) {
		t.Errorf("And(on,off) should be false")
	}
	if !CompositeOr(on, off).ShouldSample(SampleContext{}) {
		t.Errorf("Or(on,off) should be true")
	}
	if !CompositeAnd(on, on).ShouldSample(SampleContext{}) {
		t.Errorf("And(on,on) should be true")
	}
}

func TestParentBased(t *testing.T) {
	root := AlwaysOffSampler{}
	s := NewParentBasedSampler(root)
	if s.ShouldSample(SampleContext{HasParent: true}) != true {
		t.Errorf("with parent should sample")
	}
	if s.ShouldSample(SampleContext{}) != false {
		t.Errorf("root with off should not sample")
	}
}

func TestTraceIDRatioClamps(t *testing.T) {
	zero := NewTraceIDRatioBased(-1)
	if zero.ShouldSample(SampleContext{TraceID: "x"}) {
		t.Errorf("negative ratio should clamp to 0")
	}
	two := NewTraceIDRatioBased(2)
	if !two.ShouldSample(SampleContext{TraceID: "x"}) {
		t.Errorf("ratio >1 should clamp to 1")
	}
}
