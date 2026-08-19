package otlp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestExportInjectsTraceparent confirms the exporter attaches a W3C
// traceparent header when ctx carries a Trace context. Without it the
// collector cannot stitch the export into a caller trace.
func TestExportInjectsTraceparent(t *testing.T) {
	var gotTP string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTP = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(Response{})
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL, WithTimeout(0))
	ctx := context.Background()
	ctx = WithTraceID(ctx, "4bf92f3577b34da6a3ce929d0e0e4736")
	ctx = WithParentSpanID(ctx, "00f067aa0ba902b7")
	ctx = WithSampled(ctx, true)
	if _, err := exp.Export(ctx, Request{ResourceAttrs: map[string]string{"service.name": "x"}}); err != nil {
		t.Fatal(err)
	}
	if gotTP == "" {
		t.Fatal("expected traceparent header, got none")
	}
	if !strings.HasPrefix(gotTP, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-") {
		t.Fatalf("unexpected traceparent: %q", gotTP)
	}
}
