package api

import (
	"fmt"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// seed 使用的服务图，使 service-map 视图实际能够
// 展示 caller → callee 边。每个 chain 是一个顶层 service
//（例如 checkout），它向下游一系列 helper 发起调用。
var seedChains = [][]string{
	{"checkout", "auth", "postgres"},
	{"checkout", "inventory", "postgres"},
	{"checkout", "payments", "stripe"},
	{"search", "elasticsearch"},
	{"recommend", "embeddings", "vector-db"},
	{"ads", "auction", "postgres"},
	{"auth", "oauth", "postgres"},
	{"inventory", "postgres"},
}

// generateSeed 为某个 service 合成一小批 OTLP 记录。
// 由 /api/seed 和 /api/seed/stream 用于引导 demo。
//
// 当请求的 service 是某条 chain 的入口点时，
// 会产出一条遍历 chain 中每一环的 span 树，
// 使 service-map 获得逼真的 caller → callee 边。当 service
// 不在任何 chain 中时，我们回退到一个自引用的 root+child 对。
func (s *Server) generateSeed(service string, n int) model.OTLPRequest {
	now := time.Now()
	logs := make([]model.LogRecord, 0, n*2)
	metrics := make([]model.MetricPoint, 0, n*2)
	spans := make([]model.SpanRecord, 0, n*4)

	sampleLogs := []string{
		"request completed",
		"user authenticated",
		"cache miss",
		"upstream timeout, retrying",
		"queue depth high: 1024",
		"invalid input",
		"database connection reset",
		"feature flag toggled",
	}
	severities := []model.Severity{
		model.SeverityInfo, model.SeverityInfo, model.SeverityInfo,
		model.SeverityDebug, model.SeverityWarn, model.SeverityError, model.SeverityError,
	}
	metricNames := []string{
		"http.server.duration",
		"http.server.requests",
		"process.cpu",
		"system.mem.used",
	}

	// 选择以请求 service 开头的第一条 chain，否则
	// 回退到一条 2 环合成 chain，使 trace 仍具有深度。
	chain := []string{service, service + "-dep"}
	for _, c := range seedChains {
		if c[0] == service {
			chain = c
			break
		}
	}

	// 将 `n` 个事件均匀铺开到 5 分钟窗口内，使 QPS / latency
	// 窗口能呈现有意义的速率，而不是所有事件都集中在
	// 同一秒。上限为 5 分钟，使较老的数据落在
	// 默认的 hot tier 之外。
	span := 5 * time.Minute
	step := time.Second
	if n > 1 {
		step = span / time.Duration(n)
		if step < 250*time.Millisecond {
			step = 250 * time.Millisecond
		}
	}

	for i := 0; i < n; i++ {
		t := now.Add(-time.Duration(i) * step)
		traceID := fmt.Sprintf("%016x", s.randInt64())

		// 构造一条 span chain；第一条是根，随后每一条
		// span 都是前一条的子 span，并在不同的
		// service 中运行，使 service-map 能够派生跨服务边。
		var prevSpan string
		for chainIdx, svc := range chain {
			spanID := fmt.Sprintf("%016x", s.randInt64())
			startOffset := time.Duration(chainIdx) * 3 * time.Millisecond
			dur := int64(s.randFloat(2, 80)) + int64(chainIdx*30)
			status := "ok"
			if svc == "payments" || svc == "stripe" {
				if s.randFloat(0, 1) < 0.25 {
					status = "error"
				}
			}
			spans = append(spans, model.SpanRecord{
				TraceID:    traceID,
				SpanID:     spanID,
				ParentID:   prevSpan,
				Name:       endpointName(svc, chainIdx),
				Service:    svc,
				StartTime:  t.Add(startOffset),
				DurationMs: dur,
				Status:     status,
				Attributes: map[string]string{
					"env":     "demo",
					"host":    fmt.Sprintf("pod-%d", s.randintInt(64)),
					"chain":   fmt.Sprintf("%d/%d", chainIdx+1, len(chain)),
				},
			})
			prevSpan = spanID

		// 第一个链路还会写入请求日志条目与指标。
			if chainIdx == 0 {
				logs = append(logs, model.LogRecord{
					Timestamp: t,
					Service:   svc,
					Severity:  severities[s.randintInt(len(severities))],
					Body:      sampleLogs[s.randintInt(len(sampleLogs))],
					TraceID:   traceID,
					SpanID:    spanID,
					Attributes: map[string]string{
						"env":  "demo",
						"host": fmt.Sprintf("pod-%d", s.randintInt(64)),
					},
				})
				for _, name := range metricNames {
					metrics = append(metrics, model.MetricPoint{
						Timestamp: t,
						Service:   svc,
						Name:      name,
						Value:     s.randFloat(0, 1000),
						Unit:      "ms",
						Type:      "gauge",
						Labels:    map[string]string{"env": "demo"},
					})
				}
			}
		}
	}
	return model.OTLPRequest{
		ResourceAttrs: map[string]string{
			"service.name":    service,
			"service.version": "0.0.0-demo",
			"deployment.env":  "demo",
		},
		Logs:    logs,
		Metrics: metrics,
		Spans:   spans,
	}
}

// endpointName 为 chain 中的每一环返回更有意思的操作名，使
// service-detail 下钻视图能够展示逼真的端点行。
func endpointName(svc string, idx int) string {
	if idx == 0 {
		return "GET /" + svc
	}
	names := []string{
		"lookup", "auth.check", "query", "fetch", "validate",
		"call", "invoke", "rpc", "read", "write",
	}
	return svc + "." + names[idx%len(names)]
}
