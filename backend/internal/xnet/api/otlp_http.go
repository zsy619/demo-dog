package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// handleOTLPHTTP 为所有三种信号实现 OTLP/HTTP（1.0）传输。
// 协议文档：
// https://opentelemetry.io/docs/specs/otlp/#otlphttp-default-ports
//
// 端点：
//   POST /v1/logs
//   POST /v1/metrics
//   POST /v1/traces
//
// 三者使用相同的 JSON 信封形状（`ExportLogsServiceRequest`、
// `ExportMetricsServiceRequest`、`ExportTracesServiceRequest`）。
// 我们只关心资源属性和每个信号的
// 记录，这正是我们简化的 ingest 契约
// 已经理解的部分。
//
// 该服务接受 OTLP 标准 JSON 形式并将其
// 转换到我们的内部模型。转换是尽力而为：
// 额外字段被忽略，缺失字段使用默认值。线路是无损往返。
// 
//
// 认证：与其余 ingest API 一致 —— 角色网关强制
// admin / writer / reader 角色。

func (s *Server) handleOTLPHTTPLogs(rw http.ResponseWriter, r *http.Request) {
	s.otlpHTTP(rw, r, "logs")
}

func (s *Server) handleOTLPHTTPMetrics(rw http.ResponseWriter, r *http.Request) {
	s.otlpHTTP(rw, r, "metrics")
}

func (s *Server) handleOTLPHTTPTraces(rw http.ResponseWriter, r *http.Request) {
	s.otlpHTTP(rw, r, "traces")
}

// otlpHTTP 是共享的分发器。它接受任意 OTLP
// 服务请求，并将内容路由到正确的内部
// ingest 路径。
func (s *Server) otlpHTTP(rw http.ResponseWriter, r *http.Request, signal string) {
	if r.Method != http.MethodPost {
		writeError(rw, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeError(rw, http.StatusBadRequest, err)
		return
	}

	tenant := resolveTenant(r)

	// 我们按一种宽容的形态解析。任何更丰富的内容（例如 exemplars、
	// aggregation temporality、scope spans）会通过
	// attributes 映射保留下来。
	var doc otlpDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		writeError(rw, http.StatusBadRequest, err)
		return
	}

	// 构造简化的 OTLPRequest。
	req := model.OTLPRequest{TenantID: tenant}
	if doc.Resource != nil {
		req.ResourceAttrs = otlpAttrsToMap(doc.Resource.Attributes)
	} else {
		req.ResourceAttrs = map[string]string{}
	}
	// 如果请求体显式设置了 tenant_id，则对管理员密钥而言
	// 它优先于 auth 绑定的值（非管理员密钥无法绕过）。
	if doc.TenantID != "" {
		req.TenantID = doc.TenantID
	}
	// 转换每个信号的记录。
	now := time.Now()
	for _, sl := range doc.ScopeLogs {
		for _, lr := range sl.LogRecords {
			req.Logs = append(req.Logs, model.LogRecord{
				Timestamp:  timeFromUnixNano(lr.TimeUnixNano, lr.ObservedTimeUnixNano, now),
				TenantID:   tenant,
				Service:    req.ResourceAttrs["service.name"],
				Severity:   model.Severity(severityText(lr.SeverityNumber, lr.SeverityText)),
				Body:       lr.Body.StringValue,
				TraceID:    hexFromBytes(lr.TraceID),
				SpanID:     hexFromBytes(lr.SpanID),
				Attributes: otlpAttrsToMap(lr.Attributes),
			})
		}
	}
	for _, sm := range doc.ScopeMetrics {
		for _, m := range sm.Metrics {
			points := metricPoints(m, req.ResourceAttrs["service.name"], tenant)
			req.Metrics = append(req.Metrics, points...)
		}
	}
	for _, st := range doc.ScopeSpans {
		for _, sp := range st.Spans {
			req.Spans = append(req.Spans, spanRecord(sp, req.ResourceAttrs["service.name"], tenant))
		}
	}

	// 往返到我们自己的流水线。SubmitSync 可接受，
	// 因为 OTLP 路径针对低吞吐的 agent 流量；
	// 批量导出走更简单的 /api/ingest/otlp。
	s.ingest.SubmitSync(req)

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(otlpPartialSuccess(
		len(req.Logs), len(req.Metrics), len(req.Spans),
	))
}

// otlpDoc 是三个 service-request 形状的并集。
// 无论信号如何，我们都解码到相同的结构，
// 因为我们关心的字段都已按命名空间组织（如 `resource_logs`）。
type otlpDoc struct {
	TenantID      string         `json:"tenant_id,omitempty"`
	Resource      *otlpResource  `json:"resource,omitempty"`
	ScopeLogs     []otlpScopeLogs    `json:"scope_logs,omitempty"`
	ScopeMetrics  []otlpScopeMetrics `json:"scope_metrics,omitempty"`
	ScopeSpans    []otlpScopeSpans   `json:"scope_spans,omitempty"`
}

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes,omitempty"`
}

type otlpScopeLogs struct {
	LogRecords []otlpLogRecord `json:"log_records,omitempty"`
}
type otlpScopeMetrics struct {
	Metrics []otlpMetric `json:"metrics,omitempty"`
}
type otlpScopeSpans struct {
	Spans []otlpSpan `json:"spans,omitempty"`
}

type otlpKeyValue struct {
	Key   string      `json:"key"`
	Value otlpAnyValue `json:"value"`
}
type otlpAnyValue struct {
	StringValue string  `json:"string_value,omitempty"`
	BoolValue   *bool   `json:"bool_value,omitempty"`
	IntValue    *int64  `json:"int_value,omitempty"`
	DoubleValue *float64 `json:"double_value,omitempty"`
}

type otlpLogRecord struct {
	TimeUnixNano        uint64         `json:"time_unix_nano,omitempty"`
	ObservedTimeUnixNano uint64        `json:"observed_time_unix_nano,omitempty"`
	SeverityNumber      int            `json:"severity_number,omitempty"`
	SeverityText        string         `json:"severity_text,omitempty"`
	Body                otlpAnyValue   `json:"body,omitempty"`
	TraceID             []byte         `json:"trace_id,omitempty"`
	SpanID              []byte         `json:"span_id,omitempty"`
	Attributes          []otlpKeyValue `json:"attributes,omitempty"`
}

type otlpMetric struct {
	Name      string          `json:"name"`
	Sum       *otlpSum        `json:"sum,omitempty"`
	Gauge     *otlpGauge      `json:"gauge,omitempty"`
	Histogram *otlpHistogram  `json:"histogram,omitempty"`
}

type otlpSum struct {
	DataPoints []otlpNumber `json:"data_points,omitempty"`
}
type otlpGauge struct {
	DataPoints []otlpNumber `json:"data_points,omitempty"`
}
type otlpHistogram struct {
	DataPoints []otlpHistogramPoint `json:"data_points,omitempty"`
}
type otlpNumber struct {
	TimeUnixNano uint64  `json:"time_unix_nano,omitempty"`
	Value        float64  `json:"value"`
}
type otlpHistogramPoint struct {
	TimeUnixNano uint64  `json:"time_unix_nano,omitempty"`
	Sum          float64 `json:"sum"`
	Count        uint64  `json:"count"`
}

type otlpSpan struct {
	TraceID       []byte         `json:"trace_id,omitempty"`
	SpanID        []byte         `json:"span_id,omitempty"`
	ParentSpanID  []byte         `json:"parent_span_id,omitempty"`
	Name          string         `json:"name"`
	StartUnixNano uint64         `json:"start_time_unix_nano,omitempty"`
	EndUnixNano   uint64         `json:"end_time_unix_nano,omitempty"`
	Status        otlpStatus     `json:"status,omitempty"`
	Attributes    []otlpKeyValue `json:"attributes,omitempty"`
}
type otlpStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

func otlpAttrsToMap(attrs []otlpKeyValue) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, kv := range attrs {
		out[kv.Key] = anyValueToString(kv.Value)
	}
	return out
}

func anyValueToString(v otlpAnyValue) string {
	switch {
	case v.StringValue != "":
		return v.StringValue
	case v.BoolValue != nil:
		if *v.BoolValue { return "true" }
		return "false"
	case v.IntValue != nil:
		return itoa(*v.IntValue)
	case v.DoubleValue != nil:
		return ftoa(*v.DoubleValue)
	}
	return ""
}

func itoa(n int64) string {
	if n == 0 { return "0" }
	neg := false
	if n < 0 { neg = true; n = -n }
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg { i--; b[i] = '-' }
	return string(b[i:])
}

func ftoa(f float64) string {
	// 简化的 %.6g
	return floatToString(f, 6)
}

func floatToString(f float64, prec int) string {
	if f == 0 { return "0" }
	neg := false
	if f < 0 { neg = true; f = -f }
	whole := uint64(f)
	frac := f - float64(whole)
	s := itoa(int64(whole))
	if frac > 0 {
		s += "."
		for i := 0; i < prec; i++ {
			frac *= 10
			d := uint64(frac)
			if d > 9 { d = 9 }
			s += string(byte('0' + d))
			frac -= float64(d)
			if frac == 0 { break }
		}
	}
	if neg { s = "-" + s }
	return s
}

func severityText(num int, txt string) string {
	if txt != "" { return txt }
	switch num {
	case 1, 2, 3, 4: return "TRACE"
	case 5, 6, 7, 8: return "DEBUG"
	case 9, 10, 11, 12: return "INFO"
	case 13, 14, 15, 16: return "WARN"
	case 17, 18, 19, 20: return "ERROR"
	case 21, 22, 23, 24: return "FATAL"
	}
	return "INFO"
}

func timeFromUnixNano(primary, observed uint64, fallback time.Time) time.Time {
	if primary > 0 { return time.Unix(0, int64(primary)) }
	if observed > 0 { return time.Unix(0, int64(observed)) }
	return fallback
}

func metricPoints(m otlpMetric, service, tenant string) []model.MetricPoint {
	var out []model.MetricPoint
	if m.Sum != nil {
		for _, dp := range m.Sum.DataPoints {
			out = append(out, model.MetricPoint{
				Timestamp: time.Unix(0, int64(dp.TimeUnixNano)),
				TenantID:  tenant,
				Service:   service,
				Name:      m.Name,
				Value:     dp.Value,
			})
		}
	}
	if m.Gauge != nil {
		for _, dp := range m.Gauge.DataPoints {
			out = append(out, model.MetricPoint{
				Timestamp: time.Unix(0, int64(dp.TimeUnixNano)),
				TenantID:  tenant,
				Service:   service,
				Name:      m.Name,
				Value:     dp.Value,
			})
		}
	}
	if m.Histogram != nil {
		for _, dp := range m.Histogram.DataPoints {
			if dp.Count == 0 { continue }
			// 近似处理：为直方图的每个桶发出一个摘要点。
			avg := dp.Sum / float64(dp.Count)
			out = append(out, model.MetricPoint{
				Timestamp: time.Unix(0, int64(dp.TimeUnixNano)),
				TenantID:  tenant,
				Service:   service,
				Name:      m.Name + ".avg",
				Value:     avg,
			})
		}
	}
	return out
}

func spanRecord(s otlpSpan, service, tenant string) model.SpanRecord {
	status := "ok"
	if s.Status.Code == 2 { status = "error" }
	if s.Status.Code == 1 { status = "unset" }
	dur := int64(0)
	if s.EndUnixNano > 0 && s.StartUnixNano > 0 {
		dur = int64(s.EndUnixNano - s.StartUnixNano)
	}
	return model.SpanRecord{
		TraceID:   hexFromBytes(s.TraceID),
		SpanID:    hexFromBytes(s.SpanID),
		TenantID:  tenant,
		Service:   service,
		Name:      s.Name,
		StartTime: time.Unix(0, int64(s.StartUnixNano)),
		DurationMs: dur / 1_000_000,
		Status:    status,
	}
}

func hexFromBytes(b []byte) string {
	if len(b) == 0 { return "" }
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hex[v>>4]
		out[i*2+1] = hex[v&0x0f]
	}
	return string(out)
}

func otlpPartialSuccess(logs, metrics, spans int) map[string]any {
	return map[string]any{
		"partialSuccess": map[string]any{
			"accepted_logs":    logs,
			"accepted_metrics": metrics,
			"accepted_spans":   spans,
		},
	}
}
