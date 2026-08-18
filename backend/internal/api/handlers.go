package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/ingest"
	"github.com/zsy619/demo-dog/backend/internal/model"
	"github.com/zsy619/demo-dog/backend/internal/store"
	"github.com/zsy619/demo-dog/backend/internal/stream"
)

// parseFilter builds a store.QueryFilter from URL query parameters.
// Repeated "label" params (label=key=value) build a label matcher.
// Severity accepts either an exact value ("ERROR") or an at-least
// comparison ("severity>=WARN") which surfaces everything from that
// level up — this mirrors LogQL/PromQL ergonomics for the frontend.
func parseFilter(q url.Values) store.QueryFilter {
	sev := q.Get("severity")
	f := store.QueryFilter{
		Service:  q.Get("service"),
		Name:     q.Get("name"),
		Severity: sev,
		TraceID:  q.Get("trace_id"),
		Search:   q.Get("search"),
		Window:   q.Get("window"),
		Limit:    atoiDefault(q.Get("limit"), 200),
		SinceMs:  int64(atoiDefault(q.Get("since"), 0)),
		UntilMs:  int64(atoiDefault(q.Get("until"), 0)),
	}
	if strings.HasPrefix(sev, ">=") {
		rest := strings.TrimPrefix(sev, ">=")
		f.SeverityAtLeast = true
		f.Severity = ""
		f.MinSeverity = model.Severity(strings.TrimSpace(rest)).Rank()
		if f.MinSeverity < 0 {
			f.MinSeverity = 0
		}
	}
	for _, raw := range q["label"] {
		// each value is "key=value"
		for i := 0; i < len(raw); i++ {
			if raw[i] == '=' {
				if f.Labels == nil {
					f.Labels = map[string]string{}
				}
				f.Labels[raw[:i]] = raw[i+1:]
				break
			}
		}
	}
	return f
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	f := parseFilter(q)
	switch typ {
	case "logs":
		writeJSON(w, http.StatusOK, s.store.QueryLogsFiltered(f))
	case "metrics":
		writeJSON(w, http.StatusOK, s.store.QueryMetricsFiltered(f))
	case "traces":
		writeJSON(w, http.StatusOK, s.store.QueryTracesFiltered(f))
	default:
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown query type: %s", typ))
	}
}

func (s *Server) handleDataSources(w http.ResponseWriter, r *http.Request) {
	sources := s.datasources.List()
	writeJSON(w, http.StatusOK, map[string]any{
		"datasources": sources,
		"count":       len(sources),
	})
}

func (s *Server) handleDashboards(w http.ResponseWriter, r *http.Request) {
	boards := []map[string]any{
		{"id": "overview", "name": "Service Overview", "description": "QPS, p99, error rate per service.", "tags": []string{"default", "core"}},
		{"id": "logs", "name": "Logs Explorer", "description": "Tail logs across all services.", "tags": []string{"logs"}},
		{"id": "traces", "name": "Distributed Tracing", "description": "Trace waterfall.", "tags": []string{"traces"}},
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboards": boards})
}

func (s *Server) handleDashboardsPanels(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/dashboards/")
	id = strings.TrimSuffix(id, "/panels")
	panels := s.panelsFor(id)
	if panels == nil {
		writeError(w, http.StatusNotFound, errors.New("dashboard not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dashboard_id": id,
		"panels":       panels,
	})
}

func (s *Server) panelsFor(id string) []map[string]any {
	switch id {
	case "overview":
		return []map[string]any{
			{"id": "qps", "type": "timeseries", "title": "Request Rate (QPS)", "datasource": "doris", "query": "SELECT * FROM metrics WHERE name='http.server.duration_count'", "config": map[string]any{"metric": "http.server.duration", "window": "1m", "max_rows": 50}, "grid": map[string]int{"x": 0, "y": 0, "w": 12, "h": 8}},
			{"id": "p99", "type": "timeseries", "title": "Latency p99 (ms)", "datasource": "doris", "query": "SELECT * FROM metrics WHERE name='http.server.duration_p99'", "config": map[string]any{"metric": "http.server.duration", "window": "1m", "max_rows": 50}, "grid": map[string]int{"x": 12, "y": 0, "w": 12, "h": 8}},
			{"id": "errors", "type": "stat", "title": "Error Rate", "datasource": "doris", "query": "SELECT * FROM logs WHERE severity='ERROR' LIMIT 50", "config": map[string]any{"severity": "ERROR", "max_rows": 50}, "grid": map[string]int{"x": 0, "y": 8, "w": 6, "h": 4}},
			{"id": "active", "type": "stat", "title": "Active Services", "datasource": "doris", "query": "SELECT count(*) FROM services", "config": map[string]any{"metric": "http.server.requests", "max_rows": 1}, "grid": map[string]int{"x": 6, "y": 8, "w": 6, "h": 4}},
			{"id": "logs", "type": "logs", "title": "Live logs", "datasource": "doris", "query": "SELECT * FROM logs ORDER BY ts DESC LIMIT 100", "config": map[string]any{"max_rows": 100}, "grid": map[string]int{"x": 0, "y": 12, "w": 24, "h": 8}},
		}
	case "logs":
		return []map[string]any{
			{"id": "all", "type": "logs", "title": "Logs (all services)", "datasource": "doris", "query": "SELECT * FROM logs ORDER BY ts DESC LIMIT 200"},
		}
	case "traces":
		return []map[string]any{
			{"id": "recent", "type": "traces", "title": "Recent traces", "datasource": "doris", "query": "SELECT * FROM traces ORDER BY start_time DESC LIMIT 100"},
		}
	}
	return nil
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errors.New("POST only"))
		return
	}
	// Cap ingest body size to prevent a single huge payload from
	// exhausting server memory. Default 4 MiB is generous for an
	// OTel-style batch (typical is <100 KiB) but small enough to
	// not let an attacker OOM the collector.
	const maxBodyBytes = 4 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer r.Body.Close()

	// Pick decoder based on content-type. Default to our simplified JSON;
	// the OTLP/JSON envelope is used by OTel SDKs and exporters.
	var req model.OTLPRequest
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json+otlp") || strings.Contains(r.URL.Path, "otlp-json") {
		req, err = ingest.DecodeOTLPJSON(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	} else {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
			return
		}
	}
	norm := s.ingest.Normalize(&req)
	if err := s.ingest.Validate(&norm); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if !s.ingest.Submit(norm) {
		// Ingest pipeline is at capacity. We previously degraded to a
		// synchronous write here, which would block the HTTP handler
		// goroutine and stall every other request behind it. That is a
		// classic latency-explosion failure mode under load. Instead we
		// return 503 + Retry-After so the upstream SDK can back off and
		// retry. The synchronous path is still reachable via a separate
		// explicit "force=true" query param for debug use only.
		if r.URL.Query().Get("force") == "true" {
			resp := s.ingest.SubmitSync(norm)
			resp.RetryLogs += len(norm.Logs)
			resp.RetryMetrics += len(norm.Metrics)
			resp.RetrySpans += len(norm.Spans)
			writeJSON(w, http.StatusAccepted, resp)
			return
		}
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":         "ingest pipeline at capacity",
			"retry_logs":    len(norm.Logs),
			"retry_metrics": len(norm.Metrics),
			"retry_spans":   len(norm.Spans),
		})
		return
	}

	for _, l := range norm.Logs {
		s.hub.Publish(stream.Event{Kind: "log", Service: l.Service, Timestamp: l.Timestamp.UnixMilli(), Body: l.Body, TraceID: l.TraceID, SpanID: l.SpanID, Status: string(l.Severity)})
	}
	for _, m := range norm.Metrics {
		s.hub.Publish(stream.Event{Kind: "metric", Service: m.Service, Timestamp: m.Timestamp.UnixMilli(), Name: m.Name, Value: m.Value})
	}
	for _, sp := range norm.Spans {
		s.hub.Publish(stream.Event{Kind: "span", Service: sp.Service, Timestamp: sp.StartTime.UnixMilli(), Name: sp.Name, TraceID: sp.TraceID, SpanID: sp.SpanID, Status: sp.Status})
	}

	writeJSON(w, http.StatusAccepted, model.OTLPResponse{
		AcceptedLogs:    len(norm.Logs),
		AcceptedMetrics: len(norm.Metrics),
		AcceptedSpans:   len(norm.Spans),
	})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	raw, conn, err := stream.Upgrade(w, r, s.allowedOrigins)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer raw.Close()
	defer conn.Close()

	ch, cancel := s.hub.Subscribe()
	defer cancel()

	jsonRaw := helloFrame()
	welcome := jsonRaw
	_ = conn.WriteText(welcome)

	go func() {
		for {
			if _, err := conn.ReadFrame(); err != nil {
				return
			}
		}
	}()

	for ev := range ch {
		buf, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		if err := conn.WriteText(buf); err != nil {
			return
		}
	}
}

func (s *Server) handleSeed(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	if svc == "" {
		svc = "demo"
	}
	n := atoiDefault(r.URL.Query().Get("n"), 5)
	req := s.generateSeed(svc, n)
	s.ingest.SubmitSync(req)
	for _, m := range req.Metrics {
		s.hub.Publish(stream.Event{Kind: "metric", Service: m.Service, Timestamp: m.Timestamp.UnixMilli(), Name: m.Name, Value: m.Value})
	}
	for _, l := range req.Logs {
		s.hub.Publish(stream.Event{Kind: "log", Service: l.Service, Timestamp: l.Timestamp.UnixMilli(), Body: l.Body, Status: string(l.Severity)})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service": svc,
		"seeded":  n,
	})
}

func (s *Server) handleSeedStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	services := []string{"checkout", "search", "inventory", "auth", "recommend", "ads"}
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		svc := services[s.randintInt(len(services))]
		req := s.generateSeed(svc, 3)
		s.ingest.SubmitSync(req)
		for _, l := range req.Logs {
			s.hub.Publish(stream.Event{Kind: "log", Service: l.Service, Timestamp: l.Timestamp.UnixMilli(), Body: l.Body, Status: string(l.Severity)})
		}
		for _, m := range req.Metrics {
			s.hub.Publish(stream.Event{Kind: "metric", Service: m.Service, Timestamp: m.Timestamp.UnixMilli(), Name: m.Name, Value: m.Value})
		}
		buf, _ := json.Marshal(map[string]any{
			"service": svc,
			"events":  len(req.Logs) + len(req.Metrics) + len(req.Spans),
		})
		fmt.Fprintf(w, "data: %s\n\n", buf)
		flusher.Flush()
		time.Sleep(1 * time.Second)
	}
}

func (s *Server) handleRecentPayloads(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"payloads": s.ingest.RecentPayloads(),
	})
}

// handleLabelKeys returns every label key observed across the in-memory tables.
func (s *Server) handleLabelKeys(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.LabelKeys())
}

// handleServiceMap walks spans and returns caller -> callee edges.
func (s *Server) handleServiceMap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ServiceMap())
}

// handleTrace returns all spans belonging to a single trace.
func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/traces/")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("trace id required"))
		return
	}
	spans := s.store.TraceSpans(id)
	if len(spans) == 0 {
		writeError(w, http.StatusNotFound, fmt.Errorf("trace not found: %s", id))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"trace_id": id,
		"spans":    spans,
	})
}

// handleQPS returns per-service request rate series.
func (s *Server) handleQPS(w http.ResponseWriter, r *http.Request) {
	window := time.Duration(atoiDefault(r.URL.Query().Get("window_min"), 5)) * time.Minute
	series := s.store.QPSByService(window)
	out := make([]map[string]any, 0, len(series))
	for svc, pts := range series {
		out = append(out, map[string]any{
			"service": svc,
			"points":  pts,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["service"].(string) < out[j]["service"].(string) })
	writeJSON(w, http.StatusOK, map[string]any{
		"window_min": int(window.Minutes()),
		"series":     out,
	})
}

// handleHistogram returns a log-binned latency histogram for a service.
func (s *Server) handleHistogram(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	bins := atoiDefault(r.URL.Query().Get("bins"), 20)
	counts := s.store.HistogramCounts(svc, bins)
	p50, p95, p99 := s.store.PercentileLatencies(svc)
	writeJSON(w, http.StatusOK, map[string]any{
		"service": svc,
		"bins":    bins,
		"counts":  counts,
		"p50_ms":  p50,
		"p95_ms":  p95,
		"p99_ms":  p99,
	})
}

// handleHistogramOTel returns the aggregated OTel histogram (explicit
// bucket bounds + per-bucket counts) for a metric, along with p50/p95/p99
// computed from those buckets. Returns 404 when no histogram data has
// been received for the (service, name) pair, so the frontend can fall
// back to the synthetic log-binned view.
//
//   GET /api/histogram/otel?service=checkout&name=http.duration_ms
//   {
//     "service": "checkout",
//     "name": "http.duration_ms",
//     "bounds": [0.005, 0.01, ..., 10, +Inf],
//     "counts": [...],
//     "total": 1234,
//     "sum":   42.5,
//     "min":   0.001,
//     "max":   8.2,
//     "p50":   0.045,
//     "p95":   0.3,
//     "p99":   1.1
//   }
func (s *Server) handleHistogramOTel(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	name := r.URL.Query().Get("name")
	if svc == "" || name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("service and name are required"))
		return
	}
	snap := s.store.HistogramSnapshot(svc, name)
	if snap == nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("no histogram data for service=%s name=%s", svc, name))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service": svc,
		"name":    name,
		"bounds":  snap.Bounds,
		"counts":  snap.Counts,
		"total":   snap.Total,
		"sum":     snap.Sum,
		"min":     snap.Min,
		"max":     snap.Max,
		"p50":     s.store.HistogramQuantile(svc, name, 0.50),
		"p95":     s.store.HistogramQuantile(svc, name, 0.95),
		"p99":     s.store.HistogramQuantile(svc, name, 0.99),
	})
}

// handleSeverity returns a count per severity for a service (or all).
func (s *Server) handleSeverity(w http.ResponseWriter, r *http.Request) {
	svc := r.URL.Query().Get("service")
	writeJSON(w, http.StatusOK, map[string]any{
		"service": svc,
		"counts":  s.store.SeverityCounts(svc),
	})
}

// handleSnapshot returns the last N records for live-tail rendering.
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	logs, metrics, spans := s.store.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"logs":    logs,
		"metrics": metrics,
		"spans":   spans,
	})
}

// handleMetricNames returns the top metric names observed.
func (s *Server) handleMetricNames(w http.ResponseWriter, r *http.Request) {
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	writeJSON(w, http.StatusOK, map[string]any{
		"names": s.store.TopMetricNames(limit),
	})
}

// handleExport returns CSV or JSON for a query result.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	fmtName := q.Get("format")
	f := parseFilter(q)
	var rows []model.Row
	switch typ {
	case "logs":
		res := s.store.QueryLogsFiltered(f)
		rows = res.Rows
	case "traces":
		res := s.store.QueryTracesFiltered(f)
		rows = res.Rows
	default:
		writeError(w, http.StatusBadRequest, errors.New("export supports type=logs|traces"))
		return
	}
	if fmtName == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%d.csv", typ, time.Now().Unix()))
		csv.NewWriter(w).WriteAll(rowsToCSV(rows))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%d.json", typ, time.Now().Unix()))
	writeJSON(w, http.StatusOK, map[string]any{
		"type":     typ,
		"exported": len(rows),
		"rows":     rows,
	})
}

// rowsToCSV flattens a slice of rows into a CSV.
func rowsToCSV(rows []model.Row) [][]string {
	if len(rows) == 0 {
		return [][]string{}
	}
	cols := []string{}
	seen := map[string]bool{}
	for k := range rows[0] {
		if !seen[k] {
			cols = append(cols, k)
			seen[k] = true
		}
	}
	sort.Strings(cols)
	out := [][]string{cols}
	for _, r := range rows {
		line := make([]string, len(cols))
		for i, c := range cols {
			v := r[c]
			if v == nil {
				line[i] = ""
				continue
			}
			if m, ok := v.(map[string]string); ok {
				parts := make([]string, 0, len(m))
				keys := make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					parts = append(parts, fmt.Sprintf("%s=%s", k, m[k]))
				}
				line[i] = strings.Join(parts, "; ")
				continue
			}
			line[i] = fmt.Sprintf("%v", v)
		}
		out = append(out, line)
	}
	return out
}

// handlePromMetrics exports engine counters in Prometheus exposition format.
func (s *Server) handlePromMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	stats := s.store.Stats()
	poolStats := s.ingest.PoolStats()
	fmt.Fprintf(w, "# HELP dog_logs_accepted_total Number of log records accepted since start.\n")
	fmt.Fprintf(w, "# TYPE dog_logs_accepted_total counter\n")
	fmt.Fprintf(w, "dog_logs_accepted_total %d\n", stats.LogsAccepted)
	fmt.Fprintf(w, "# HELP dog_metrics_accepted_total Number of metric points accepted since start.\n")
	fmt.Fprintf(w, "# TYPE dog_metrics_accepted_total counter\n")
	fmt.Fprintf(w, "dog_metrics_accepted_total %d\n", stats.MetricsAccepted)
	fmt.Fprintf(w, "# HELP dog_spans_accepted_total Number of spans accepted since start.\n")
	fmt.Fprintf(w, "# TYPE dog_spans_accepted_total counter\n")
	fmt.Fprintf(w, "dog_spans_accepted_total %d\n", stats.SpansAccepted)
	fmt.Fprintf(w, "# HELP dog_queries_served_total Number of read queries served.\n")
	fmt.Fprintf(w, "# TYPE dog_queries_served_total counter\n")
	fmt.Fprintf(w, "dog_queries_served_total %d\n", stats.QueriesServed)
	fmt.Fprintf(w, "# HELP dog_hot_rows Number of rows in the hot tier per signal.\n")
	fmt.Fprintf(w, "# TYPE dog_hot_rows gauge\n")
	fmt.Fprintf(w, "dog_hot_rows{signal=\"logs\"} %d\n", stats.HotLogs)
	fmt.Fprintf(w, "dog_hot_rows{signal=\"metrics\"} %d\n", stats.HotMetrics)
	fmt.Fprintf(w, "dog_hot_rows{signal=\"spans\"} %d\n", stats.HotSpans)
	fmt.Fprintf(w, "# HELP dog_cold_rows Number of rows in the cold tier per signal.\n")
	fmt.Fprintf(w, "# TYPE dog_cold_rows gauge\n")
	fmt.Fprintf(w, "dog_cold_rows{signal=\"logs\"} %d\n", stats.ColdLogs)
	fmt.Fprintf(w, "dog_cold_rows{signal=\"metrics\"} %d\n", stats.ColdMetrics)
	fmt.Fprintf(w, "dog_cold_rows{signal=\"spans\"} %d\n", stats.ColdSpans)
	fmt.Fprintf(w, "# HELP dog_uptime_seconds Process uptime in seconds.\n")
	fmt.Fprintf(w, "# TYPE dog_uptime_seconds gauge\n")
	fmt.Fprintf(w, "dog_uptime_seconds %.0f\n", time.Since(s.started).Seconds())

	// Ingest pipeline counters — operators need these to spot back-pressure.
	fmt.Fprintf(w, "# HELP dog_ingest_jobs_accepted_total Jobs enqueued to the worker pool.\n")
	fmt.Fprintf(w, "# TYPE dog_ingest_jobs_accepted_total counter\n")
	fmt.Fprintf(w, "dog_ingest_jobs_accepted_total %d\n", poolStats.Accepted)
	fmt.Fprintf(w, "# HELP dog_ingest_jobs_processed_total Jobs completed (success or terminal failure).\n")
	fmt.Fprintf(w, "# TYPE dog_ingest_jobs_processed_total counter\n")
	fmt.Fprintf(w, "dog_ingest_jobs_processed_total %d\n", poolStats.Processed)
	fmt.Fprintf(w, "# HELP dog_ingest_jobs_retried_total Jobs retried after transient failure.\n")
	fmt.Fprintf(w, "# TYPE dog_ingest_jobs_retried_total counter\n")
	fmt.Fprintf(w, "dog_ingest_jobs_retried_total %d\n", poolStats.Retried)
	fmt.Fprintf(w, "# HELP dog_ingest_jobs_failed_total Jobs that exhausted retries.\n")
	fmt.Fprintf(w, "# TYPE dog_ingest_jobs_failed_total counter\n")
	fmt.Fprintf(w, "dog_ingest_jobs_failed_total %d\n", poolStats.Failed)

	// Go runtime metrics — minimal set so an operator can see
	// goroutine / heap pressure without pulling in a full Prometheus
	// client library.
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, "# HELP dog_go_goroutines Number of goroutines that currently exist.\n")
	fmt.Fprintf(w, "# TYPE dog_go_goroutines gauge\n")
	fmt.Fprintf(w, "dog_go_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "# HELP dog_go_memstats_alloc_bytes Bytes of allocated heap objects.\n")
	fmt.Fprintf(w, "# TYPE dog_go_memstats_alloc_bytes gauge\n")
	fmt.Fprintf(w, "dog_go_memstats_alloc_bytes %d\n", ms.Alloc)
	fmt.Fprintf(w, "# HELP dog_go_memstats_sys_bytes Total bytes of memory obtained from the OS.\n")
	fmt.Fprintf(w, "# TYPE dog_go_memstats_sys_bytes gauge\n")
	fmt.Fprintf(w, "dog_go_memstats_sys_bytes %d\n", ms.Sys)
	fmt.Fprintf(w, "# HELP dog_go_memstats_gc_pause_total_seconds Cumulative GC pause time.\n")
	fmt.Fprintf(w, "# TYPE dog_go_memstats_gc_pause_total_seconds counter\n")
	fmt.Fprintf(w, "dog_go_memstats_gc_pause_total_seconds %.6f\n", float64(ms.PauseTotalNs)/1e9)
}

// helloFrame returns a pre-encoded welcome frame for new websocket clients.
func helloFrame() []byte {
	return []byte{0x7b, 0x22, 0x6b, 0x69, 0x6e, 0x64, 0x22, 0x3a, 0x22, 0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x22, 0x2c, 0x22, 0x74, 0x72, 0x61, 0x63, 0x65, 0x22, 0x3a, 0x22, 0x44, 0x4f, 0x47, 0x22, 0x7d}
}
