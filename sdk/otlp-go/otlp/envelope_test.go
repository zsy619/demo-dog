package otlp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEncodeOTLPEnvelope(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	req := Request{
		ResourceAttrs: map[string]string{"service.name": "x"},
		Logs: []LogRecord{{
			Timestamp: now,
			Severity:   SeverityInfo,
			Body:       "hello",
			Attributes: map[string]string{"k": "v"},
			TraceID:    "00000000000000000000000000000001",
			SpanID:     "0000000000000001",
		}},
		Metrics: []MetricPoint{{
			Timestamp: now,
			Name:      "m",
			Value:     42,
			Type:      TypeCounter,
			Labels:    map[string]string{"l": "x"},
		}},
		Spans: []SpanRecord{{
			TraceID:    "00000000000000000000000000000001",
			SpanID:     "0000000000000001",
			Name:       "s",
			StartTime:  now,
			DurationMs: 5,
			Status:     StatusOK,
			Links: []SpanLink{{
				TraceID: "00000000000000000000000000000002",
				SpanID:  "0000000000000002",
			}},
		}},
	}
	body, err := EncodeOTLPEnvelope(req)
	if err != nil {
		t.Fatal(err)
	}
	var env OTelEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, body)
	}
	if len(env.ResourceLogs) != 1 || len(env.ResourceLogs[0].ScopeLogs) != 1 || len(env.ResourceLogs[0].ScopeLogs[0].LogRecords) != 1 {
		t.Fatalf("logs shape: %+v", env.ResourceLogs)
	}
	lr := env.ResourceLogs[0].ScopeLogs[0].LogRecords[0]
	if lr.SeverityText != "INFO" || lr.SeverityNumber != 9 {
		t.Fatalf("severity: %+v", lr)
	}
	if lr.Body["stringValue"] != "hello" {
		t.Fatalf("body: %+v", lr.Body)
	}
	if len(env.ResourceMetrics) != 1 || len(env.ResourceMetrics[0].ScopeMetrics[0].Metrics) != 1 {
		t.Fatalf("metrics shape: %+v", env.ResourceMetrics)
	}
	m := env.ResourceMetrics[0].ScopeMetrics[0].Metrics[0]
	if m.Sum == nil || m.Gauge != nil {
		t.Fatalf("counter should be sum: %+v", m)
	}
	if m.Sum.DataPoints[0].AsDouble != 42 {
		t.Fatalf("value: %+v", m.Sum.DataPoints[0])
	}
	if len(env.ResourceSpans) != 1 || len(env.ResourceSpans[0].ScopeSpans[0].Spans) != 1 {
		t.Fatalf("spans shape: %+v", env.ResourceSpans)
	}
	sp := env.ResourceSpans[0].ScopeSpans[0].Spans[0]
	if sp.Status.Code != 1 {
		t.Fatalf("status code: %d", sp.Status.Code)
	}
	if len(sp.Links) != 1 || sp.Links[0].TraceID != "00000000000000000000000000000002" {
		t.Fatalf("links: %+v", sp.Links)
	}
}

func TestOTelExporterRoundTrip(t *testing.T) {
	var seenContentType string
	var seenBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenContentType = r.Header.Get("Content-Type")
		seenBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(Response{AcceptedSpans: 1})
	}))
	defer srv.Close()

	exp := NewOTelExporter(srv.URL)
	now := time.Now()
	resp, err := exp.Export(context.Background(), Request{
		ResourceAttrs: map[string]string{"service.name": "x"},
		Spans: []SpanRecord{{
			TraceID:    "00000000000000000000000000000001",
			SpanID:     "0000000000000001",
			Name:       "s",
			StartTime:  now,
			DurationMs: 1,
			Status:     StatusOK,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AcceptedSpans != 1 {
		t.Fatalf("accepted: %+v", resp)
	}
	if !strings.HasPrefix(seenContentType, "application/json+otlp") {
		t.Fatalf("content-type: %s", seenContentType)
	}
	if len(seenBody) == 0 || !strings.Contains(string(seenBody), "resourceSpans") {
		t.Fatalf("body did not contain resourceSpans: %s", seenBody)
	}
}

func TestOTelSeverityNumber(t *testing.T) {
	cases := map[string]int{
		"TRACE": 1, "DEBUG": 5, "INFO": 9,
		"WARN": 13, "ERROR": 17, "FATAL": 21,
		"unknown": 9,
	}
	for s, want := range cases {
		if got := OTelSeverityNumber(s); got != want {
			t.Errorf("OTelSeverityNumber(%q) = %d, want %d", s, got, want)
		}
	}
}
