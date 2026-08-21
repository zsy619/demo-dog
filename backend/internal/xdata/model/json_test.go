package model

// json_test.go:验证 SDK 简化信封与后端规范信封都能正确解码。

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLogRecordUnmarshal_BackendStandard(t *testing.T) {
	body := []byte(`{"timestamp":"2024-01-15T10:30:00.123456789Z","service":"checkout","severity":"WARN","body":"hello"}`)
	var l LogRecord
	if err := json.Unmarshal(body, &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if l.Service != "checkout" || l.Severity != SeverityWarn || l.Body != "hello" {
		t.Errorf("unexpected: %+v", l)
	}
	if l.Timestamp.Year() != 2024 || l.Timestamp.Month() != time.January {
		t.Errorf("timestamp wrong: %v", l.Timestamp)
	}
}

func TestLogRecordUnmarshal_SDKNodeStyle(t *testing.T) {
	// Node SDK sends timestamp_ns + severity_text
	body := []byte(`{"timestamp_ns":1700000000000000000,"severity_text":"INFO","body":"hi","attributes":{"k":"v"}}`)
	var l LogRecord
	if err := json.Unmarshal(body, &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if l.Severity != SeverityInfo {
		t.Errorf("severity: %q", l.Severity)
	}
	if l.Attributes["k"] != "v" {
		t.Errorf("attrs: %v", l.Attributes)
	}
	if l.Timestamp.UnixNano() != 1700000000000000000 {
		t.Errorf("ts: %v", l.Timestamp)
	}
}

func TestLogRecordUnmarshal_SDKPythonStyle(t *testing.T) {
	// Python SDK uses timestamp_ns as int(time.time()*1e9)
	body := []byte(`{"timestamp_ns":1700000000000000000,"severity_text":"ERROR","body":"x"}`)
	var l LogRecord
	if err := json.Unmarshal(body, &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if l.Severity != SeverityError {
		t.Errorf("severity: %q", l.Severity)
	}
}

func TestMetricPointUnmarshal_SDKStyle(t *testing.T) {
	// Python SDK: timestamp in ms. After ms-band auto-detection,
	// 1735689600000 is treated as ms → 1735689600000 ms = Jan 2025.
	body := []byte(`{"timestamp":1735689600000,"name":"requests","value":42,"type":"counter"}`)
	var m MetricPoint
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Name != "requests" || m.Value != 42 {
		t.Errorf("unexpected: %+v", m)
	}
	if got := m.Timestamp.UnixMilli(); got != 1735689600000 {
		t.Errorf("ts unix_ms=%d want 1735689600000", got)
	}
}

func TestSpanRecordUnmarshal_SDKStyle(t *testing.T) {
	// Java SDK style: start_unix_nano + duration_ns + parent_span_id
	body := []byte(`{"trace_id":"t1","span_id":"s1","parent_span_id":"s0","service":"checkout","name":"GET","start_unix_nano":1700000000000000000,"duration_ns":50000000,"status":"ok"}`)
	var s SpanRecord
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TraceID != "t1" || s.SpanID != "s1" {
		t.Errorf("ids: %v", s)
	}
	if s.ParentID != "s0" {
		t.Errorf("parent: %q", s.ParentID)
	}
	if s.DurationMs != 50 {
		t.Errorf("duration_ms: %d", s.DurationMs)
	}
}

func TestParseNumber_AutoDetect(t *testing.T) {
	// Each value falls cleanly into a magnitude band; the expected
	// unix seconds are derived by treating n as the unit itself.
	cases := []struct {
		in   int64
		want int64 // expected unix seconds
	}{
		{1700000000, 1700000000},               // 1.7e9 s
		{1735689600000, 1735689600},            // 1.735e12 ms = 1.735e9 s
		{1700000000000000, 1700000000},         // 1.7e15 us = 1.7e9 s
		{1700000000000000000, 1700000000},      // 1.7e18 ns = 1.7e9 s
	}
	for _, tc := range cases {
		got := parseNumber(tc.in).Unix()
		if got != tc.want {
			t.Errorf("parseNumber(%d) unix=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestLogRecordUnmarshal_BadTimestamp(t *testing.T) {
	body := []byte(`{"timestamp":"not-a-date","service":"a","body":"b"}`)
	var l LogRecord
	err := json.Unmarshal(body, &l)
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Errorf("expected timestamp error, got %v", err)
	}
}
