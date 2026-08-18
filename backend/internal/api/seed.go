package api

import (
	"fmt"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/model"
)

// Service graph used by the seed so the service-map view actually
// shows caller → callee edges. Each chain is a top-level service
// (e.g. checkout) that issues calls into a chain of helpers.
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

// generateSeed synthesizes a small batch of OTLP records for a service.
// Used by /api/seed and /api/seed/stream to bootstrap the demo.
//
// When the requested service is the entry point of a chain (e.g. "checkout"),
// the seed produces a span tree that walks through every link of the chain
// so the service-map gets realistic caller → callee edges. When the service
// is not in any chain, we fall back to a self-referential root+child pair.
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

	// Pick the first chain that starts with the requested service, else
	// fall back to a 2-link synthetic chain so the trace still has depth.
	chain := []string{service, service + "-dep"}
	for _, c := range seedChains {
		if c[0] == service {
			chain = c
			break
		}
	}

	// Spread `n` events evenly across a 5-minute window so QPS / latency
	// windows pick up a meaningful rate instead of all events landing in
	// the same second. Capped at 5m so older data falls outside the
	// default hot tier.
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

		// Build a chain of spans; the first is the root, each subsequent
		// span is a child of the previous one and runs in a different
		// service so the service-map can derive cross-service edges.
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

			// The first link also gets the request log entry + metrics.
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

// endpointName returns a more interesting operation name per chain link so
// the service-detail drill-down shows realistic endpoint rows.
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
