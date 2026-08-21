package model

// sdk_e2e_test.go: 用真实 SDK 风格 JSON 直接走模型的反序列化,
// 模拟 ingest 链路,确认 SDK 数据被后端正确识别。
//
// 备注:本测试只验证模型层 —— 缺失 service/timestamp/severity 等字段
// 由 Ingestor.Normalize() 在 ingest 链路兜底填充(见 ingest/otlp_test.go)。

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRealSDKPayload_NodeJS(t *testing.T) {
	// 这是 Node SDK 实际发送的 envelope: log/metric/span 都用 timestamp_ns + severity_text
	body := []byte(`{
    "resource_attrs": {
      "service.name": "checkout",
      "service.version": "1.2.3"
    },
    "tenant_id": "acme",
    "logs": [
      {"timestamp_ns": 1700000000000000000, "severity_text": "INFO", "body": "user signed in", "attributes": {"user": "alice"}}
    ],
    "metrics": [
      {"timestamp": 1700000000000, "name": "orders_placed", "value": 1, "attributes": {"region": "us"}}
    ],
    "spans": [
      {"trace_id": "t1", "span_id": "s1", "parent_span_id": "s0", "service": "checkout", "name": "GET /x", "start_unix_nano": 1700000000000000000, "duration_ns": 50000000, "status": "ok"}
    ]
  }`)
	var req struct {
		TenantID      string            `json:"tenant_id,omitempty"`
		ResourceAttrs map[string]string `json:"resource_attrs"`
		Logs          []LogRecord       `json:"logs,omitempty"`
		Metrics       []MetricPoint     `json:"metrics,omitempty"`
		Spans         []SpanRecord      `json:"spans,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.Logs) != 1 || req.Logs[0].Severity != SeverityInfo || req.Logs[0].Body != "user signed in" {
		t.Errorf("logs: %+v", req.Logs)
	}
	if len(req.Metrics) != 1 || req.Metrics[0].Name != "orders_placed" || req.Metrics[0].Value != 1 {
		t.Errorf("metrics: %+v", req.Metrics)
	}
	if len(req.Spans) != 1 || req.Spans[0].TraceID != "t1" || req.Spans[0].ParentID != "s0" || req.Spans[0].DurationMs != 50 {
		t.Errorf("spans: %+v", req.Spans)
	}
	// 资源属性正常解析
	if req.ResourceAttrs["service.name"] != "checkout" {
		t.Errorf("resource_attrs: %v", req.ResourceAttrs)
	}
}

func TestRealSDKPayload_PythonPy(t *testing.T) {
	body := []byte(`{
    "resource_attrs": {"service.name": "payments"},
    "tenant_id": "acme",
    "logs": [
      {"timestamp_ns": 1700000005000000000, "severity_text": "WARN", "body": "slow query", "attributes": {"sql_id": "42"}}
    ]
  }`)
	var req struct {
		Logs []LogRecord `json:"logs"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Logs[0].Severity != SeverityWarn {
		t.Errorf("severity: %q", req.Logs[0].Severity)
	}
	if req.Logs[0].Attributes["sql_id"] != "42" {
		t.Errorf("attrs: %v", req.Logs[0].Attributes)
	}
}

func TestRealSDKPayload_Java(t *testing.T) {
	// Java SDK 真实 payload
	body := []byte(`{
    "resource_attrs": {"service.name": "inventory"},
    "spans": [
      {"trace_id":"abc","span_id":"01","parent_span_id":"","service":"inventory","name":"PUT /stock","start_unix_nano":1700000010000000000,"duration_ns":75000000,"status":"ok"}
    ]
  }`)
	var req struct {
		Spans []SpanRecord `json:"spans"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Spans[0].Service != "inventory" {
		t.Errorf("service: %q", req.Spans[0].Service)
	}
	if req.Spans[0].DurationMs != 75 {
		t.Errorf("duration_ms: %d", req.Spans[0].DurationMs)
	}
}

func TestRejectNonStringTimestamp(t *testing.T) {
	body := []byte(`{"timestamp": "definitely-not-a-date", "body":"x"}`)
	var l LogRecord
	err := json.Unmarshal(body, &l)
	if err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Errorf("expected timestamp parse error, got %v", err)
	}
}
