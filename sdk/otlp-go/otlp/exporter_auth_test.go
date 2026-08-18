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

func TestExporter_APIKeyBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"accepted_logs":0,"accepted_metrics":1,"accepted_spans":0}`)
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL, WithAPIKey("secret-token"))
	if _, err := exp.Export(context.Background(), Request{Metrics: []MetricPoint{{Name: "x"}}}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("expected Bearer header, got %q", gotAuth)
	}
}

func TestExporter_NoKeyNoHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"accepted_logs":0,"accepted_metrics":1,"accepted_spans":0}`)
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL)
	if _, err := exp.Export(context.Background(), Request{Metrics: []MetricPoint{{Name: "x"}}}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no Authorization header, got %q", gotAuth)
	}
}

func TestExporter_EmptyKeyIgnored(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"accepted_logs":0,"accepted_metrics":1,"accepted_spans":0}`)
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL, WithAPIKey(""))
	if _, err := exp.Export(context.Background(), Request{Metrics: []MetricPoint{{Name: "x"}}}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no Authorization header for empty key, got %q", gotAuth)
	}
}

func TestSDK_WithAuthTokenPropagates(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"accepted_logs":0,"accepted_metrics":1,"accepted_spans":0}`)
	}))
	defer srv.Close()

	sdk, err := New(srv.URL,
		WithService("auth-test"),
		WithAuthToken("secret-token"),
		WithFlushInterval(time.Hour),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer sdk.Shutdown(context.Background())

	sdk.Counter(context.Background(), "x", 1)
	if err := sdk.flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("expected Bearer prefix, got %q", gotAuth)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("expected exact bearer, got %q", gotAuth)
	}
}

// touch json import to keep gofmt happy if we remove other references.
var _ = json.Marshal
