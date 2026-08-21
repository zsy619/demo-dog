package model

// json.go:为 LogRecord / MetricPoint / SpanRecord 提供宽松的 JSON 反序列化,
// 同时兼容后端的 RFC3339 + enum severity 与 SDK(Node / Python / Java)的
// ns-since-epoch + severity_text 简化信封。
//
// 这是向前兼容的扩展 —— 任何字段缺省时由 Ingestor.Normalize() 兜底填充,
// 不会拒绝写请求。

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// flexibleLogRecord 是 LogRecord 的 JSON 兼容解码形态。
type flexibleLogRecord struct {
	// 标准字段
	Timestamp  any            `json:"timestamp,omitempty"`
	TenantID   string         `json:"tenant_id,omitempty"`
	Service    string         `json:"service"`
	Severity   string         `json:"severity,omitempty"`
	Body       string         `json:"body"`
	Attributes map[string]any `json:"attributes,omitempty"`
	TraceID    string         `json:"trace_id,omitempty"`
	SpanID     string         `json:"span_id,omitempty"`

	// SDK 字段
	TimestampNs  int64          `json:"timestamp_ns,omitempty"`
	SeverityText string         `json:"severity_text,omitempty"`
	Attrs        map[string]any `json:"-"`
}

// UnmarshalJSON 接受以下任一时间戳形态:
//
//   - string RFC3339 / RFC3339Nano(后端规范)
//   - number 纳秒(ns,Node/Python/Java SDK)
//   - number 毫秒(ms,常见 Prometheus 等)
//   - number 秒(秒级时间戳,偶有 SDK 输出)
//
// Severity 同时支持 severity(枚举)与 severity_text(SDK 自由文本)。
func (l *LogRecord) UnmarshalJSON(b []byte) error {
	var f flexibleLogRecord
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	l.TenantID = f.TenantID
	l.Service = f.Service
	l.Body = f.Body
	l.TraceID = f.TraceID
	l.SpanID = f.SpanID
	l.Attributes = stringifyAttrs(f.Attributes)
	// severity 取 severity 或 severity_text
	if f.Severity != "" {
		l.Severity = Severity(strings.ToUpper(f.Severity))
	} else if f.SeverityText != "" {
		l.Severity = Severity(strings.ToUpper(f.SeverityText))
	}
	// timestamp 多形态
	ts, err := parseFlexibleTime(f.Timestamp, f.TimestampNs, "timestamp")
	if err != nil {
		return err
	}
	l.Timestamp = ts
	return nil
}

// flexibleMetricPoint 是 MetricPoint 的 JSON 兼容解码形态。
type flexibleMetricPoint struct {
	Timestamp any    `json:"timestamp,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	Service   string `json:"service"`
	Name      string `json:"name"`
	Value     float64 `json:"value"`
	Unit      string `json:"unit,omitempty"`
	Type      string `json:"type,omitempty"`
	Labels    map[string]any `json:"labels,omitempty"`
	// SDK 字段
	TimestampMs int64          `json:"timestamp_ms,omitempty"`
}

func (m *MetricPoint) UnmarshalJSON(b []byte) error {
	var f flexibleMetricPoint
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	m.TenantID = f.TenantID
	m.Service = f.Service
	m.Name = f.Name
	m.Value = f.Value
	m.Unit = f.Unit
	m.Type = f.Type
	m.Labels = stringifyAttrs(f.Labels)
	ts, err := parseFlexibleTime(f.Timestamp, f.TimestampMs*int64(time.Millisecond), "timestamp")
	if err != nil {
		return err
	}
	m.Timestamp = ts
	return nil
}

// flexibleSpanRecord 是 SpanRecord 的 JSON 兼容解码形态。
type flexibleSpanRecord struct {
	TraceID   string         `json:"trace_id"`
	SpanID    string         `json:"span_id"`
	ParentID  string         `json:"parent_id,omitempty"`
	Name      string         `json:"name"`
	TenantID  string         `json:"tenant_id,omitempty"`
	Service   string         `json:"service"`
	StartTime any            `json:"start_time,omitempty"`
	DurationMs int64         `json:"duration_ms"`
	Status    string         `json:"status,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`

	// SDK 字段(Java/Python/Node 都用 ns since epoch)
	StartUnixNano int64 `json:"start_unix_nano,omitempty"`
	DurationNs    int64 `json:"duration_ns,omitempty"`
	ParentSpanID  string `json:"parent_span_id,omitempty"`
}

func (s *SpanRecord) UnmarshalJSON(b []byte) error {
	var f flexibleSpanRecord
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	s.TraceID = f.TraceID
	s.SpanID = f.SpanID
	s.ParentID = f.ParentID
	if s.ParentID == "" {
		s.ParentID = f.ParentSpanID
	}
	s.Name = f.Name
	s.TenantID = f.TenantID
	s.Service = f.Service
	s.Status = f.Status
	s.Attributes = stringifyAttrs(f.Attributes)
	// duration
	if f.DurationMs != 0 {
		s.DurationMs = f.DurationMs
	} else if f.DurationNs != 0 {
		s.DurationMs = f.DurationNs / int64(time.Millisecond)
		if s.DurationMs == 0 && f.DurationNs > 0 {
			s.DurationMs = 1 // 纳秒级抖动:至少计 1ms 以避免 0
		}
	}
	// start_time 多形态
	ts, err := parseFlexibleTime(f.StartTime, f.StartUnixNano, "start_time")
	if err != nil {
		return err
	}
	s.StartTime = ts
	return nil
}

// parseFlexibleTime 把多种时间戳表达统一解析为 time.Time。
//
//  - raw 为 string 时尝试 RFC3339 / RFC3339Nano;
//  - raw 为 number (ns) 时直接转;
//  - raw 为 nil/空时回退到 altNs;
//  - 都缺时返回 zero time(交给 Normalize() 兜底填 now)。
func parseFlexibleTime(raw any, altNs int64, field string) (time.Time, error) {
	switch v := raw.(type) {
	case nil:
		// 回落到 altNs
	case string:
		if v == "" {
			break
		}
		// 尝试多种格式
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("%s: unparseable timestamp %q", field, v)
	case float64:
		return parseNumber(float64ToInt64(v)), nil
	case int64:
		return parseNumber(v), nil
	case int:
		return parseNumber(int64(v)), nil
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return parseNumber(n), nil
		}
		if f, err := v.Float64(); err == nil {
			return parseNumber(float64ToInt64(f)), nil
		}
	}
	if altNs != 0 {
		return parseNumber(altNs), nil
	}
	return time.Time{}, nil
}

// parseNumber 按数量级自动识别 ns / us / ms / s。
//
//   - >= 10^17 视为纳秒(典型 ~2026+ 年,Node/Python/Java SDK 风格)
//   - >= 10^15 视为微秒
//   - >= 10^12 视为毫秒(Prometheus / 部分 SDK 风格)
//   - >= 10^9  视为秒
//   - 否则视为低值纳秒
func parseNumber(n int64) time.Time {
	switch {
	case n >= 1e17:
		return time.Unix(0, n)
	case n >= 1e15:
		return time.UnixMicro(n)
	case n >= 1e12:
		return time.UnixMilli(n)
	case n >= 1e9:
		return time.Unix(n, 0)
	default:
		return time.Unix(0, n)
	}
}

func float64ToInt64(f float64) int64 {
	return int64(f)
}

// stringifyAttrs 把 map[string]any 转换为 map[string]string。
// 非字符串值会被强制转换为 string。
func stringifyAttrs(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case string:
			out[k] = t
		case nil:
			out[k] = ""
		default:
			out[k] = fmt.Sprintf("%v", t)
		}
	}
	return out
}
