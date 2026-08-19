package otlpgo

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEnvelope_ToBundle(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	env := Envelope{
		TenantID: "acme",
		Service:  "checkout",
		Spans: []Span{{
			TraceID: "abc", SpanID: "def", Name: "GET /pay",
			Start: now, End: now.Add(50 * time.Millisecond),
			Status: "ok", Attrs: ResourceAttrs{"route": "/pay"},
		}},
		Logs: []LogRecord{{
			Timestamp: now, Severity: SeverityInfo, Body: "hello",
			TraceID: "abc", SpanID: "def",
		}},
		Metrics: []MetricPoint{{
			Timestamp: now, Name: "http.server.duration",
			Value: 120.5, Attrs: ResourceAttrs{"code": "200"},
		}},
	}
	b := env.ToBundle()
	if b.TenantID != "acme" {
		t.Fatalf("tenant: %s", b.TenantID)
	}
	if len(b.Spans) != 1 || b.Spans[0].DurationMs != 50 {
		t.Fatalf("spans: %+v", b.Spans)
	}
	if len(b.Logs) != 1 || b.Logs[0].Body != "hello" {
		t.Fatalf("logs: %+v", b.Logs)
	}
	if len(b.Metrics) != 1 || b.Metrics[0].Name != "http.server.duration" {
		t.Fatalf("metrics: %+v", b.Metrics)
	}
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var back Bundle
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Spans) != 1 || back.Spans[0].TraceID != "abc" {
		t.Fatalf("roundtrip spans: %+v", back.Spans)
	}
}

func TestEnvelope_Defaults(t *testing.T) {
	b := Envelope{}.ToBundle()
	if b.TenantID != "default" {
		t.Fatalf("default tenant: %s", b.TenantID)
	}
	if b.ResourceAttrs["service.name"] != "unknown" {
		t.Fatalf("default service: %s", b.ResourceAttrs["service.name"])
	}
	if len(b.Spans) != 0 || len(b.Logs) != 0 || len(b.Metrics) != 0 {
		t.Fatal("empty envelope should yield empty slices")
	}
}

func TestSeverityMapping(t *testing.T) {
	cases := map[Severity]string{
		SeverityTrace: "DEBUG",
		SeverityDebug: "DEBUG",
		SeverityInfo:  "INFO",
		SeverityWarn:  "WARN",
		SeverityError: "ERROR",
		SeverityFatal: "FATAL",
		0:             "INFO",
	}
	for sev, want := range cases {
		b := Envelope{
			Service: "s",
			Logs:    []LogRecord{{Severity: sev, Body: "x"}},
		}.ToBundle()
		if b.Logs[0].Severity != want {
			t.Errorf("sev=%d got=%s want=%s", sev, b.Logs[0].Severity, want)
		}
	}
}
