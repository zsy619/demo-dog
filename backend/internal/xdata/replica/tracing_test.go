package replica

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubTracer struct {
	tp, ts string
	span  int
}

func (s *stubTracer) Header(_ context.Context) (string, string) {
	return s.tp, s.ts
}

func (s *stubTracer) NewSpan(_ context.Context, _ string) (context.Context, string, string) {
	return context.Background(), s.tp, s.ts
}

func TestTraceRT_InjectsHeader(t *testing.T) {
	var gotTP, gotTS string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTP = r.Header.Get("traceparent")
		gotTS = r.Header.Get("tracestate")
		w.WriteHeader(200)
	}))
	defer ts.Close()
	tr := &stubTracer{tp: "00-aaaa-bbbb-01", ts: "vendor=value"}
	client := &http.Client{Transport: NewTraceRT(nil, tr)}
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotTP != "00-aaaa-bbbb-01" {
		t.Fatalf("traceparent: %s", gotTP)
	}
	if gotTS != "vendor=value" {
		t.Fatalf("tracestate: %s", gotTS)
	}
}

func TestTraceRT_NoTracer(t *testing.T) {
	var gotTP string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTP = r.Header.Get("traceparent")
		w.WriteHeader(200)
	}))
	defer ts.Close()
	client := &http.Client{Transport: NewTraceRT(nil, nil)}
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotTP != "" {
		t.Fatalf("no tracer should not inject: %s", gotTP)
	}
}

func TestTraceRT_TracerEmptyHeader(t *testing.T) {
	var gotTP string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTP = r.Header.Get("traceparent")
		w.WriteHeader(200)
	}))
	defer ts.Close()
	tr := &stubTracer{} // returns "" for header
	client := &http.Client{Transport: NewTraceRT(nil, tr)}
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotTP != "" {
		t.Fatalf("empty traceparent should not inject: %s", gotTP)
	}
}

func TestExtractTrace_Valid(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	tid, sid, fl := ExtractTrace(r)
	if tid != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("tid: %s", tid)
	}
	if sid != "00f067aa0ba902b7" {
		t.Fatalf("sid: %s", sid)
	}
	if fl != "01" {
		t.Fatalf("flags: %s", fl)
	}
}

func TestExtractTrace_Missing(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	tid, sid, fl := ExtractTrace(r)
	if tid != "" || sid != "" || fl != "" {
		t.Fatal("expected empty for missing")
	}
}

func TestExtractTrace_TooShort(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("traceparent", "00-aaaa-bbbb")
	tid, sid, fl := ExtractTrace(r)
	if tid != "" || sid != "" || fl != "" {
		t.Fatal("expected empty for short")
	}
}

func TestExtractTrace_Garbage(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("traceparent", "not-a-valid-traceparent")
	tid, _, _ := ExtractTrace(r)
	if tid != "" {
		t.Fatalf("garbage should be empty: %s", tid)
	}
}

func TestWithTraceparent(t *testing.T) {
	ctx := WithTraceparent(context.Background(), "00-abc-def-01")
	if got := TraceFromContext(ctx); got != "00-abc-def-01" {
		t.Fatalf("got: %s", got)
	}
}

func TestTraceFromContext_Nil(t *testing.T) {
	if got := TraceFromContext(nil); got != "" {
		t.Fatal("nil ctx should return empty")
	}
}

func TestTraceFromContext_NoKey(t *testing.T) {
	if got := TraceFromContext(context.Background()); got != "" {
		t.Fatal("missing key should return empty")
	}
}
