package api

import (
	"bytes"
	"strings"
	"testing"
)

func TestHistogram_ObserveAndWrite(t *testing.T) {
	h := newHistogram(map[string]string{"method": "GET", "route": "/api/health"}, []float64{0.01, 0.1, 1})
	h.Observe(0.005)
	h.Observe(0.05)
	h.Observe(5)

	var buf bytes.Buffer
	h.WriteText(&buf, "dog_test")
	out := buf.String()
	if !strings.Contains(out, `dog_test_count{method="GET",route="/api/health"} 3`) {
		t.Errorf("count line missing or wrong:\n%s", out)
	}
	if !strings.Contains(out, `le="+Inf"`) {
		t.Errorf("+Inf bucket missing:\n%s", out)
	}
}

func TestHistogramVec_WithLabelValues(t *testing.T) {
	v := newHistogramVec([]float64{0.01, 0.1, 1})
	h1 := v.WithLabelValues("method", "GET", "route", "/a")
	h2 := v.WithLabelValues("method", "GET", "route", "/a")
	if h1 != h2 {
		t.Errorf("expected same series for same labels")
	}
	h3 := v.WithLabelValues("method", "POST", "route", "/a")
	if h1 == h3 {
		t.Errorf("expected different series for different labels")
	}
}

func TestTrimRoute_CollapsesNoise(t *testing.T) {
	in := "/api/services/checkout"
	out := trimRoute(in)
	// "services" should NOT be collapsed; only hex-like long IDs and
	// anything that isn't the literal "api" segment are candidates.
	// We just assert the route stays understandable.
	if !strings.HasPrefix(out, "/api/") {
		t.Errorf("prefix lost: %q", out)
	}

	long := "/api/traces/0123456789abcdef0123456789abcdef"
	out2 := trimRoute(long)
	if !strings.Contains(out2, "{id}") {
		t.Errorf("expected hex id to collapse: %q", out2)
	}
}
