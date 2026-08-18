package otlp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func startAcceptedServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("{\"accepted_logs\":1,\"accepted_metrics\":1,\"accepted_spans\":1}"))
	}))
	return srv
}

func TestStatsCounters(t *testing.T) {
	srv := startAcceptedServer(t)
	defer srv.Close()

	sdk, err := New(srv.URL,
		WithService("stats-test"),
		WithFlushInterval(0),
		WithMaxBatch(100),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	sdk.Log(context.Background(), SeverityInfo, "x")
	sdk.Counter(context.Background(), "m", 1)
	_, end := sdk.Trace(context.Background(), "op")
	end(nil)
	if err := sdk.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}

	st := sdk.Stats()
	if st.LogsEmitted == 0 {
		t.Errorf("logs: %d", st.LogsEmitted)
	}
	if st.MetricsEmitted == 0 {
		t.Errorf("metrics: %d", st.MetricsEmitted)
	}
	if st.SpansEmitted == 0 {
		t.Errorf("spans: %d", st.SpansEmitted)
	}
	if st.FlushCalls == 0 {
		t.Errorf("flush_calls: %d", st.FlushCalls)
	}
}

func TestStatsSnapshot(t *testing.T) {
	st := Stats{}
	st.LogsEmitted.Add(7)
	snap := st.Snapshot()
	if snap.LogsEmitted != 7 {
		t.Fatalf("snapshot: %+v", snap)
	}
}

func TestStatsSamplerSkipped(t *testing.T) {
	sdk, _ := New("http://127.0.0.1:1",
		WithService("stats-test"),
		WithFlushInterval(0),
		WithSampler(AlwaysOffSampler{}),
	)
	defer sdk.Shutdown(context.Background())

	_, end := sdk.Trace(context.Background(), "op")
	end(nil)

	st := sdk.Stats()
	if st.SamplerSkipped == 0 {
		t.Errorf("sampler_skipped: %d", st.SamplerSkipped)
	}
	if st.SpansEmitted != 0 {
		t.Errorf("spans should be 0 (sampled off): %d", st.SpansEmitted)
	}
}

func TestStatsFlushErrors(t *testing.T) {
	sdk, _ := New("http://127.0.0.1:1",
		WithService("stats-test"),
		WithFlushInterval(0),
		WithMaxBatch(1),
	)
	defer sdk.Shutdown(context.Background())

	for i := 0; i < 5; i++ {
		sdk.Log(context.Background(), SeverityInfo, "x")
	}
	_ = sdk.ForceFlush(context.Background())

	st := sdk.Stats()
	if st.FlushCalls == 0 {
		t.Errorf("flush_calls: %d", st.FlushCalls)
	}
	if st.FlushErrors == 0 {
		t.Errorf("flush_errors: %d", st.FlushErrors)
	}
	if st.RequeuedLogs == 0 {
		t.Errorf("requeued_logs: %d", st.RequeuedLogs)
	}
}

func TestWithErrorHandler(t *testing.T) {
	var captured error
	sdk, _ := New("http://127.0.0.1:1",
		WithService("stats-test"),
		WithFlushInterval(0),
		WithErrorHandler(func(err error) { captured = err }),
	)
	defer sdk.Shutdown(context.Background())

	sdk.Log(context.Background(), SeverityInfo, "x")
	_ = sdk.ForceFlush(context.Background())
	if captured == nil {
		t.Fatalf("error handler not invoked")
	}
}
