package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

func TestWAL_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.bin")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	now := time.Now()
	w.Append(opLog, []model.LogRecord{
		{Timestamp: now, TenantID: "t1", Service: "checkout", Severity: model.SeverityInfo, Body: "hello"},
	})
	w.Append(opMetric, []model.MetricPoint{
		{Timestamp: now, TenantID: "t1", Service: "checkout", Name: "cpu", Value: 0.42},
	})
	w.Append(opSpan, []model.SpanRecord{
		{TraceID: "abc", SpanID: "01", TenantID: "t1", Service: "checkout", Name: "GET /x", StartTime: now, Status: "ok"},
	})
	logs, metrics, spans, err := w.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || len(metrics) != 1 || len(spans) != 1 {
		t.Fatalf("replay lost records: logs=%d metrics=%d spans=%d", len(logs), len(metrics), len(spans))
	}
}

func TestWAL_RepairTruncatedFrame(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.bin")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	w.Append(opLog, []model.LogRecord{{Timestamp: time.Now(), TenantID: "t1", Service: "x", Severity: "INFO", Body: "first"}})
	w.Append(opLog, []model.LogRecord{{Timestamp: time.Now(), TenantID: "t1", Service: "x", Severity: "INFO", Body: "second"}})
	// Append a truncated frame manually.
	_, _ = w.f.Write([]byte{0x00, 0x0D, 0x06, 0x00, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 16, 0, 'A'})
	_ = w.Close()
	w2, err := OpenWAL(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	logs, _, _, err := w2.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 records after repair, got %d", len(logs))
	}
	_ = w2.Close()
}

func TestWAL_Rotate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.bin")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	w.Append(opLog, []model.LogRecord{{Timestamp: time.Now(), TenantID: "t1", Service: "x", Body: "keep"}})
	if err := w.Rotate(); err != nil {
		t.Fatal(err)
	}
	logs, _, _, _ := w.Replay()
	if len(logs) != 0 {
		t.Fatalf("expected empty replay after rotate, got %d", len(logs))
	}
	_ = w.Close()
}

func TestWAL_PersistAndReplay(t *testing.T) {
	dir := t.TempDir()
	w, err := OpenWAL(filepath.Join(dir, "wal.bin"))
	if err != nil {
		t.Fatal(err)
	}
	d := New(DefaultConfig())
	d.SetWAL(w)
	d.InsertLogs([]model.LogRecord{{Timestamp: time.Now(), TenantID: "t1", Service: "checkout", Severity: model.SeverityInfo, Body: "replay-me"}})
	d.InsertMetrics([]model.MetricPoint{{Timestamp: time.Now(), TenantID: "t1", Service: "checkout", Name: "cpu", Value: 0.7}})
	d.InsertSpans([]model.SpanRecord{{TraceID: "tid", SpanID: "s1", TenantID: "t1", Service: "checkout", Name: "GET /", StartTime: time.Now(), Status: "ok"}})
	_ = w.Close()

	// Simulate restart.
	w2, _ := OpenWAL(filepath.Join(dir, "wal.bin"))
	d2 := New(DefaultConfig())
	d2.SetWAL(w2)
	if err := d2.ReplayInto(w2); err != nil {
		t.Fatal(err)
	}
	svcs := d2.ListServices("")
	if len(svcs) != 1 || svcs[0].Name != "checkout" {
		t.Fatalf("replay did not restore services: %v", svcs)
	}
	_ = w2.Close()

	// Clean up so the test does not leave a stray file.
	_ = os.RemoveAll(dir)
}
