package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/model"
)

func TestSnapshot_RoundTrip(t *testing.T) {
	d := New(DefaultConfig())
	d.InsertLogs([]model.LogRecord{
		{Timestamp: time.Now(), Service: "checkout", Severity: model.SeverityInfo, Body: "hello"},
	})
	d.InsertMetrics([]model.MetricPoint{
		{Timestamp: time.Now(), Service: "checkout", Name: "rps", Value: 1, Type: "counter"},
	})

	data, err := d.PersistSnapshotBytes()
	if err != nil {
		t.Fatalf("PersistSnapshotBytes: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty snapshot")
	}

	d2 := New(DefaultConfig())
	if err := d2.RestoreSnapshot(bytes.NewReader(data)); err != nil {
		t.Fatalf("RestoreSnapshot: %v", err)
	}
	logs, _, _ := d2.Snapshot()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log after restore, got %d", len(logs))
	}
}

func TestSnapshot_LoadMissingFile(t *testing.T) {
	d := New(DefaultConfig())
	if err := d.LoadFromFile("/tmp/nonexistent-snapshot-" + t.Name()); err != nil {
		t.Fatalf("LoadFromFile on missing file: %v", err)
	}
}
