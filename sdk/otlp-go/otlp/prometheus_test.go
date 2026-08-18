package otlp

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPrometheusRender(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	req := Request{
		ResourceAttrs: map[string]string{"service.name": "x"},
		Metrics: []MetricPoint{
			{Timestamp: now, Name: "checkout_counter", Value: 42, Type: TypeCounter,
				Labels: map[string]string{"channel": "web"}},
			{Timestamp: now, Name: "queue.depth", Value: 100, Type: TypeGauge},
			{Timestamp: now, Name: "latency.ms", Value: 78.4, Type: TypeHistogram},
		},
	}
	out, err := NewPrometheusExporter().Render(req)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"# HELP checkout_counter",
		"# TYPE checkout_counter counter",
		"checkout_counter{\"channel\"=\"web\"} 42",
		"# TYPE queue_depth gauge",
		"queue_depth 100",
		"# TYPE latency_ms histogram",
		"latency_ms_sum 78.4",
		"latency_ms_count 1",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestPrometheusPrefix(t *testing.T) {
	now := time.Now()
	req := Request{Metrics: []MetricPoint{
		{Timestamp: now, Name: "x", Value: 1, Type: TypeGauge},
	}}
	out, _ := NewPrometheusExporter(WithPrometheusPrefix("app_")).Render(req)
	if !strings.Contains(string(out), "app_x 1") {
		t.Errorf("prefix not applied: %s", out)
	}
}

func TestPrometheusExport(t *testing.T) {
	// Export must satisfy ExporterInterface so users can wire it in.
	exp := NewPrometheusExporter()
	resp, err := exp.Export(context.Background(), Request{
		Metrics: []MetricPoint{{Name: "m", Value: 1, Type: TypeGauge}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AcceptedMetrics != 1 {
		t.Fatalf("accepted: %+v", resp)
	}
}

func TestEscapeLabelValue(t *testing.T) {
	cases := map[string]string{
		"a\"b":    "a\\\"b",
		"a\\b":     "a\\\\b",
		"a\nb":     "a\\nb",
		"normal":   "normal",
	}
	for in, want := range cases {
		if got := escapeLabelValue(in); got != want {
			t.Errorf("escapeLabelValue(%q) = %q, want %q", in, got, want)
		}
	}
}
