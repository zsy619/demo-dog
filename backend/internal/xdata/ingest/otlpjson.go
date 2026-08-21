package ingest

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// otlpJSONEnvelope 对应标准 OTLP/HTTP JSON 线上格式。
// See: https://opentelemetry.io/docs/specs/otlp/#json-protobuf-encoding
//
// We accept any subset of the three signals; missing arrays are simply empty.
type otlpJSONEnvelope struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []otlpAttr `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Scope struct {
				Name string `json:"name"`
			} `json:"scope"`
			Spans []struct {
				TraceID           string            `json:"traceId"`
				SpanID            string            `json:"spanId"`
				ParentSpanID      string            `json:"parentSpanId"`
				Name              string            `json:"name"`
				Kind              int               `json:"kind"`
				StartTimeUnixNano string            `json:"startTimeUnixNano"`
				EndTimeUnixNano   string            `json:"endTimeUnixNano"`
				Attributes        []otlpAttr        `json:"attributes"`
				Status            struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"status"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`

	ResourceMetrics []struct {
		Resource struct {
			Attributes []otlpAttr `json:"attributes"`
		} `json:"resource"`
		ScopeMetrics []struct {
			Metrics []struct {
				Name string `json:"name"`
				Unit string `json:"unit"`
				Sum  *struct {
					AggregationTemporality int                `json:"aggregationTemporality"`
					DataPoints             []otlpNumberDP      `json:"dataPoints"`
					Attributes             []otlpAttr          `json:"attributes"`
				} `json:"sum,omitempty"`
				Gauge *struct {
					DataPoints []otlpNumberDP `json:"dataPoints"`
					Attributes  []otlpAttr     `json:"attributes"`
				} `json:"gauge,omitempty"`
				Histogram *struct {
					DataPoints []struct {
						Attributes        []otlpAttr  `json:"attributes"`
						StartTimeUnixNano string      `json:"startTimeUnixNano"`
						TimeUnixNano      string      `json:"timeUnixNano"`
						Count             string      `json:"count"`
						Sum               float64     `json:"sum"`
						Min               float64     `json:"min,omitempty"`
						Max               float64     `json:"max,omitempty"`
						BucketCounts      []string    `json:"bucketCounts,omitempty"`
						ExplicitBounds    []float64   `json:"explicitBounds,omitempty"`
					} `json:"dataPoints"`
				} `json:"histogram,omitempty"`
			} `json:"metrics"`
		} `json:"scopeMetrics"`
	} `json:"resourceMetrics"`

	ResourceLogs []struct {
		Resource struct {
			Attributes []otlpAttr `json:"attributes"`
		} `json:"resource"`
		ScopeLogs []struct {
			LogRecords []struct {
				TimeUnixNano         string     `json:"timeUnixNano"`
				ObservedTimeUnixNano string     `json:"observedTimeUnixNano"`
				SeverityNumber       int        `json:"severityNumber"`
				SeverityText         string     `json:"severityText"`
				Body                 otlpAnyVal `json:"body"`
				Attributes           []otlpAttr `json:"attributes"`
				TraceID              string     `json:"traceId"`
				SpanID               string     `json:"spanId"`
			} `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}

// otlpAttr 匹配 AnyValueField 的 JSON 形态：{key, value:{stringValue|intValue|...}}
type otlpAttr struct {
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

type otlpAnyVal map[string]any

type otlpNumberDP struct {
	Attributes       []otlpAttr `json:"attributes"`
	StartTimeUnixNano string    `json:"startTimeUnixNano"`
	TimeUnixNano      string    `json:"timeUnixNano"`
	AsInt             string    `json:"asInt"`
	AsDouble          float64   `json:"asDouble"`
}

// DecodeOTLPJSON 将符合 OTel 标准的 OTLP/JSON 信封转换为我们的
// internal model. It does NOT call our internal JSON decoder -- the wire
// shape is different.
//
// 返回 (request, error). A successful decode may still produce 空的
// payload (if no signals were present); the caller should treat that as
// valid-but-empty.
func DecodeOTLPJSON(body []byte) (model.OTLPRequest, error) {
	var env otlpJSONEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return model.OTLPRequest{}, fmt.Errorf("invalid OTLP/JSON: %w", err)
	}
	out := model.OTLPRequest{
		ResourceAttrs: map[string]string{},
		Logs:          []model.LogRecord{},
		Metrics:       []model.MetricPoint{},
		Spans:         []model.SpanRecord{},
	}
	for _, rs := range env.ResourceSpans {
		resourceAttrs := attrListToMap(rs.Resource.Attributes)
		mergeAttrs(out.ResourceAttrs, resourceAttrs)
		for _, ss := range rs.ScopeSpans {
			_ = ss.Scope.Name
			for _, sp := range ss.Spans {
				start := nsToTime(sp.StartTimeUnixNano)
				end := nsToTime(sp.EndTimeUnixNano)
				dur := end.Sub(start).Milliseconds()
				status := "unset"
				if sp.Status.Code == 2 {
					status = "error"
				} else if sp.Status.Code == 1 {
					status = "ok"
				}
				attrs := attrListToMap(sp.Attributes)
				mergeAttrs(attrs, resourceAttrs)
				svc := attrs["service.name"]
				if svc == "" {
					svc = out.ResourceAttrs["service.name"]
				}
				traceID := trimHex(sp.TraceID)
				spanID := trimHex(sp.SpanID)
				parentID := trimHex(sp.ParentSpanID)
				out.Spans = append(out.Spans, model.SpanRecord{
					TraceID:    traceID,
					SpanID:     spanID,
					ParentID:   parentID,
					Name:       sp.Name,
					Service:    svc,
					StartTime:  start,
					DurationMs: dur,
					Status:     status,
					Attributes: attrs,
				})
			}
		}
	}
	for _, rl := range env.ResourceLogs {
		resourceAttrs := attrListToMap(rl.Resource.Attributes)
		mergeAttrs(out.ResourceAttrs, resourceAttrs)
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				ts := nsToTime(lr.TimeUnixNano)
				if ts.IsZero() {
					ts = nsToTime(lr.ObservedTimeUnixNano)
				}
				if ts.IsZero() {
					ts = time.Now()
				}
				sev := otlpSeverityToDog(lr.SeverityNumber, lr.SeverityText)
				attrs := attrListToMap(lr.Attributes)
				mergeAttrs(attrs, resourceAttrs)
				svc := attrs["service.name"]
				if svc == "" {
					svc = out.ResourceAttrs["service.name"]
				}
				out.Logs = append(out.Logs, model.LogRecord{
					Timestamp:  ts,
					Service:    svc,
					Severity:   sev,
					Body:       anyValToString(lr.Body),
					TraceID:    trimHex(lr.TraceID),
					SpanID:     trimHex(lr.SpanID),
					Attributes: attrs,
				})
			}
		}
	}
	for _, rm := range env.ResourceMetrics {
		resourceAttrs := attrListToMap(rm.Resource.Attributes)
		mergeAttrs(out.ResourceAttrs, resourceAttrs)
		for _, sm := range rm.ScopeMetrics {
			for _, mt := range sm.Metrics {
				unit := mt.Unit
				switch {
				case mt.Sum != nil:
					for _, dp := range mt.Sum.DataPoints {
						attrs := attrListToMap(dp.Attributes)
						mergeAttrs(attrs, resourceAttrs)
						out.Metrics = append(out.Metrics, model.MetricPoint{
							Timestamp: nsToTime(dp.TimeUnixNano),
							Service:   pickService(attrs, out.ResourceAttrs),
							Name:      mt.Name,
							Value:     numberDPValue(dp),
							Unit:      unit,
							Type:      "counter",
							Labels:    attrs,
						})
					}
				case mt.Gauge != nil:
					for _, dp := range mt.Gauge.DataPoints {
						attrs := attrListToMap(dp.Attributes)
						mergeAttrs(attrs, resourceAttrs)
						out.Metrics = append(out.Metrics, model.MetricPoint{
							Timestamp: nsToTime(dp.TimeUnixNano),
							Service:   pickService(attrs, out.ResourceAttrs),
							Name:      mt.Name,
							Value:     numberDPValue(dp),
							Unit:      unit,
							Type:      "gauge",
							Labels:    attrs,
						})
					}
				case mt.Histogram != nil:
				for _, dp := range mt.Histogram.DataPoints {
					attrs := attrListToMap(dp.Attributes)
					mergeAttrs(attrs, resourceAttrs)
					// 如果 SDK 发送了显式桶边界 + 计数，
					// pass them through so the store can compute true
					// quantiles. Otherwise fall back to the count+sum
					// pseudo-metrics that mimic OTel semantics over a
					// backend that only supports numeric points.
					if len(dp.ExplicitBounds) > 0 && len(dp.BucketCounts) > 0 {
						bounds := append([]float64(nil), dp.ExplicitBounds...)
						counts := make([]int64, len(dp.BucketCounts))
						for i, s := range dp.BucketCounts {
							counts[i] = int64(parseFloat(s))
						}
						out.Metrics = append(out.Metrics, model.MetricPoint{
							Timestamp:      nsToTime(dp.TimeUnixNano),
							Service:        pickService(attrs, out.ResourceAttrs),
							Name:           mt.Name,
							Value:          dp.Sum,
							Unit:           unit,
							Type:           "histogram",
							Labels:         attrs,
							BucketBounds:   bounds,
							BucketCounts:   counts,
							HistogramCount: int64(parseFloat(dp.Count)),
							HistogramSum:   dp.Sum,
							HistogramMin:   dp.Min,
							HistogramMax:   dp.Max,
						})
						continue
					}
					out.Metrics = append(out.Metrics, model.MetricPoint{
						Timestamp: nsToTime(dp.TimeUnixNano),
						Service:   pickService(attrs, out.ResourceAttrs),
						Name:      mt.Name + "_count",
						Value:     parseFloat(dp.Count),
						Unit:      unit,
						Type:      "histogram",
						Labels:    attrs,
					}, model.MetricPoint{
						Timestamp: nsToTime(dp.TimeUnixNano),
						Service:   pickService(attrs, out.ResourceAttrs),
						Name:      mt.Name + "_sum",
						Value:     dp.Sum,
						Unit:      unit,
						Type:      "histogram",
						Labels:    attrs,
					})
				}
				}
			}
		}
	}
	return out, nil
}

// attrListToMap 将 OTLP 属性列表拍平为字符串 map。
func attrListToMap(attrs []otlpAttr) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	out := make(map[string]string, len(attrs))
	for _, a := range attrs {
		out[a.Key] = anyValToString(otlpAnyVal(a.Value))
	}
	return out
}

// anyValToString 将 OTLP AnyValue 的任何变体转换为扁平字符串。
func anyValToString(v otlpAnyVal) string {
	if v == nil {
		return ""
	}
	for _, key := range []string{"stringValue", "intValue", "doubleValue", "boolValue"} {
		if raw, ok := v[key]; ok {
			return fmt.Sprintf("%v", raw)
		}
	}
	if arr, ok := v["arrayValue"].(map[string]any); ok {
		// 将数组拍平为逗号连接的字符串
		if vals, ok := arr["values"].([]any); ok {
			parts := make([]string, 0, len(vals))
			for _, x := range vals {
				if m, ok := x.(map[string]any); ok {
					parts = append(parts, anyValToString(otlpAnyVal(m)))
				} else {
					parts = append(parts, fmt.Sprintf("%v", x))
				}
			}
			return strings.Join(parts, ",")
		}
	}
	if kv, ok := v["kvlistValue"].(map[string]any); ok {
		if vals, ok := kv["values"].([]any); ok {
			parts := make([]string, 0, len(vals))
			for _, x := range vals {
				m, ok := x.(map[string]any)
				if !ok {
					continue
				}
				k, _ := m["key"].(string)
				parts = append(parts, fmt.Sprintf("%s=%v", k, m["value"]))
			}
			return strings.Join(parts, "; ")
		}
	}
	return fmt.Sprintf("%v", v)
}

// nsToTime 解析 OTLP 纳秒时间戳字符串。
func nsToTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// trimHex 去除 JSON 编码器可能留下的空白/前缀
// a hex-encoded trace/span id.
func trimHex(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// OTel JSON 通常使用不带 0x 前缀的小写十六进制，但也容忍 0x。
	s = strings.TrimPrefix(s, "0x")
	if _, err := hex.DecodeString(s); err != nil {
		return s // pass through; downstream may still match it
	}
	return s
}

// otlpSeverityToDog 将 OTLP 严重级（数字或文本）映射到我们的 model.Severity。
func otlpSeverityToDog(num int, text string) model.Severity {
	switch strings.ToUpper(text) {
	case "TRACE":
		return model.SeverityTrace
	case "DEBUG":
		return model.SeverityDebug
	case "INFO":
		return model.SeverityInfo
	case "WARN", "WARNING":
		return model.SeverityWarn
	case "ERROR":
		return model.SeverityError
	case "FATAL", "CRITICAL":
		return model.SeverityFatal
	}
	switch num {
	case 1, 2, 3, 4:
		return model.SeverityTrace
	case 5, 6, 7, 8:
		return model.SeverityDebug
	case 9, 10, 11, 12:
		return model.SeverityInfo
	case 13, 14, 15, 16:
		return model.SeverityWarn
	case 17, 18, 19, 20:
		return model.SeverityError
	case 21, 22, 23, 24:
		return model.SeverityFatal
	}
	return model.SeverityInfo
}

// numberDPValue 优先选择 int 表示（类似计数的指标），而非 double。
func numberDPValue(dp otlpNumberDP) float64 {
	if dp.AsInt != "" {
		if n, err := strconv.ParseInt(dp.AsInt, 10, 64); err == nil {
			return float64(n)
		}
	}
	return dp.AsDouble
}

// parseFloat 尽力从 JSON 字符串编码的数字解析 float。
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return n
	}
	return 0
}

// mergeAttrs 将 src 中的非空条目复制到 dst。
func mergeAttrs(dst, src map[string]string) {
	for k, v := range src {
		if _, ok := dst[k]; !ok && v != "" {
			dst[k] = v
		}
	}
}

// pickService 返回在任一 map 中找到的首个 service.name 值。
func pickService(attrs, resource map[string]string) string {
	if s := attrs["service.name"]; s != "" {
		return s
	}
	return resource["service.name"]
}
