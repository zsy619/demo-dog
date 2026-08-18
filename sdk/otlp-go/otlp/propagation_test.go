package otlp

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestPropagatorRoundTrip(t *testing.T) {
	prop := NewPropagator()
	sdk, err := New("http://x",
		WithService("test-prop"),
		WithFlushInterval(0),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	ctx, end := sdk.Trace(context.Background(), "test-root")
	defer end(nil)

	// Inject onto an outbound request.
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	prop.InjectHTTPHeader(ctx, req)
	tp := req.Header.Get("traceparent")
	if tp == "" {
		t.Fatal("traceparent missing after Inject")
	}

	// The injected header must round-trip through Extract.
	tc := prop.ExtractHTTPHeader(req)
	if tc == nil {
		t.Fatal("Extract returned nil")
	}
	if err := tc.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	// The extracted trace_id must match what the SDK minted for ctx.
	want := getTraceID(ctx)
	if tc.TraceID != want {
		t.Fatalf("trace_id mismatch: %q vs %q", tc.TraceID, want)
	}
}

func TestPropagatorExtractMalformed(t *testing.T) {
	prop := NewPropagator()
	cases := []string{
		"",
		"garbage",
		"00-1234-5678-99",
		"00-abc-123456789012345678901234-12-00",
	}
	for _, c := range cases {
		h := http.Header{}
		if c != "" {
			h.Set("traceparent", c)
		}
		if got := prop.Extract(h); got != nil {
			t.Errorf("Extract(%q) = %+v, want nil", c, got)
		}
	}
}

func TestPropagatorWithTraceContext(t *testing.T) {
	prop := NewPropagator()
	tc := &TraceContext{
		TraceID: "00000000000000000000000000000001",
		SpanID:  "0000000000000001",
		Flags:   "01",
		Sampled: true,
	}
	ctx := prop.WithTraceContext(context.Background(), tc)
	if got := getTraceID(ctx); got != tc.TraceID {
		t.Fatalf("trace_id: %q", got)
	}
	if got := getParentSpanID(ctx); got != tc.SpanID {
		t.Fatalf("parent span_id: %q", got)
	}
}

func TestTraceContextValidate(t *testing.T) {
	cases := []struct {
		tc   TraceContext
		want bool
	}{
		{TraceContext{TraceID: strings.Repeat("0", 32), SpanID: strings.Repeat("0", 16)}, true},
		{TraceContext{TraceID: "short", SpanID: strings.Repeat("0", 16)}, false},
		{TraceContext{TraceID: strings.Repeat("0", 32), SpanID: "short"}, false},
	}
	for i, c := range cases {
		err := c.tc.Validate()
		if (err == nil) != c.want {
			t.Errorf("case %d: validate err=%v want=%v", i, err, c.want)
		}
	}
}

func TestPropagatorInjectNoContext(t *testing.T) {
	prop := NewPropagator()
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	// No Trace was called -> ctx has no trace info; Inject must be a no-op.
	prop.InjectHTTPHeader(context.Background(), req)
	if req.Header.Get("traceparent") != "" {
		t.Fatalf("expected empty traceparent, got %q", req.Header.Get("traceparent"))
	}
}

func TestPropagatorExtractStruct(t *testing.T) {
	tc := &TraceContext{TraceID: "a", SpanID: "b", Sampled: true}
	if !reflect.DeepEqual(tc.TraceparentString(), "00-a-b-01") {
		t.Fail()
	}
}
