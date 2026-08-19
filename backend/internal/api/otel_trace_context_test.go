package api

import (
	"net/http/httptest"
	"testing"
)

func TestParseTraceContext_Valid(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	r.Header.Set("tracestate", "vendor=value")
	tc := ParseTraceContext(r)
	if tc == nil {
		t.Fatal("expected parsed context")
	}
	if tc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace id: %s", tc.TraceID)
	}
	if tc.SpanID != "00f067aa0ba902b7" {
		t.Fatalf("span id: %s", tc.SpanID)
	}
	if !tc.Sampled {
		t.Fatal("expected sampled=true")
	}
	if tc.Tracestate != "vendor=value" {
		t.Fatalf("tracestate: %s", tc.Tracestate)
	}
}

func TestParseTraceContext_Missing(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if tc := ParseTraceContext(r); tc != nil {
		t.Fatalf("expected nil, got %+v", tc)
	}
}

func TestParseTraceContext_AllZero(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("traceparent", "00-00000000000000000000000000000000-0000000000000000-01")
	if tc := ParseTraceContext(r); tc != nil {
		t.Fatalf("expected nil for all-zero, got %+v", tc)
	}
}

func TestParseTraceContext_Invalid(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("traceparent", "junk")
	if tc := ParseTraceContext(r); tc != nil {
		t.Fatalf("expected nil for invalid, got %+v", tc)
	}
}

func TestGenerateTraceContext(t *testing.T) {
	tc := GenerateTraceContext()
	if len(tc.TraceID) != 32 {
		t.Fatalf("trace id len: %d", len(tc.TraceID))
	}
	if len(tc.SpanID) != 16 {
		t.Fatalf("span id len: %d", len(tc.SpanID))
	}
	if !tc.Sampled {
		t.Fatal("expected sampled")
	}
}

func TestInjectTraceContext(t *testing.T) {
	rw := httptest.NewRecorder()
	tc := &TraceContext{
		Version: "00",
		TraceID: "abc", SpanID: "def",
		Flags: "01", Sampled: true,
	}
	InjectTraceContext(rw, tc)
	if got := rw.Header().Get("traceparent"); got != "00-abc-def-01" {
		t.Fatalf("got %q", got)
	}
}
