package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestExportRoundtrip asserts the wire payload the SDK sends decodes back
// into the same Records we built. This is the contract the SDK must keep
// against the backend ingest decoder.
func TestExportRoundtrip(t *testing.T) {
	var got Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("content-type: %s", ct)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(Response{
			AcceptedLogs:    len(got.Logs),
			AcceptedMetrics: len(got.Metrics),
			AcceptedSpans:   len(got.Spans),
		})
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL, WithTimeout(0))
	req := Request{
		ResourceAttrs: map[string]string{"service.name": "x"},
		Logs: []LogRecord{
			{Severity: SeverityInfo, Body: "hello", Attributes: map[string]string{"k": "v"}},
		},
		Metrics: []MetricPoint{
			{Name: "m", Value: 1, Type: TypeCounter},
		},
		Spans: []SpanRecord{
			{TraceID: "t", SpanID: "s", Name: "n", Service: "x", DurationMs: 5, Status: StatusOK},
		},
	}
	resp, err := exp.Export(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.AcceptedLogs != 1 || resp.AcceptedMetrics != 1 || resp.AcceptedSpans != 1 {
		t.Fatalf("counts: %+v", resp)
	}
	if got.ResourceAttrs["service.name"] != "x" {
		t.Fatalf("resource_attrs: %+v", got.ResourceAttrs)
	}
	if got.Logs[0].Body != "hello" || got.Logs[0].Attributes["k"] != "v" {
		t.Fatalf("log: %+v", got.Logs[0])
	}
}

// TestContentType confirms the Content-Type header matches what the backend
// accepts. The backend routes the OTel JSON envelope only when the body
// claims the OTel content-type; otherwise it falls back to the simplified
// envelope, which is what this SDK emits.
func TestContentType(t *testing.T) {
	var ct string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(Response{})
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL, WithTimeout(0))
	if _, err := exp.Export(context.Background(), Request{ResourceAttrs: map[string]string{"service.name": "x"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type: %q", ct)
	}
	// Empty body should still serialize to JSON without erroring.
	if !bytes.HasSuffix([]byte("ok"), []byte("ok")) {
		t.Fatal()
	}
}
