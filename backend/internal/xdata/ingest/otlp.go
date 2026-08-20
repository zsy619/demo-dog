// Package ingest implements the OTLP-style JSON ingest pipeline.
//
// The public surface is intentionally narrow: a single Ingestor struct that
// knows how to take an OTLPRequest, validate it, batch it, and finally push
// it into the in-memory Doris engine via the worker pool.
//
// Two implementation details are worth highlighting:
//
//   - We coalesce many small per-request payloads into a single batch payload
//     inside the worker, so the per-write critical section is one slice append
//     per signal rather than many independent locks.
//   - We honor `RetryLogs/Metrics/Spans` in the response so the frontend can
//     decide whether to retry or degrade the UI.
package ingest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xflow/batch"
	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
)

// Ingestor wires the OTLP decoder, the worker pool, and the store together.
type Ingestor struct {
	store *store.Doris
	pool  *batch.Pool

	// Recent payloads kept for debugging / replay.
	recentMu sync.RWMutex
	recent   []model.OTLPRequest
}

// New constructs an Ingestor with a custom pool size; default is 8 workers.
func New(s *store.Doris, poolSize int) *Ingestor {
	if poolSize <= 0 {
		poolSize = 8
	}
	pool := batch.NewPool(batch.Options{
		Workers:      poolSize,
		QueueSize:    4096,
		RetryMax:     3,
		RetryBackoff: 25 * time.Millisecond,
	})
	return &Ingestor{store: s, pool: pool}
}

// Close drains the worker pool.
func (in *Ingestor) Close() {
	in.pool.Close()
}

// PoolStats exposes the worker pool counters (accepted, processed,
// retried, failed). The HTTP /metrics handler renders them as
// Prometheus counters so operators can see ingest back-pressure in
// real time.
func (in *Ingestor) PoolStats() batch.Stats {
	return in.pool.Stats()
}

// Validate performs lightweight sanity checks on an OTLPRequest.
// It does not check every field; missing service name is the only hard error.
func (in *Ingestor) Validate(req *model.OTLPRequest) error {
	if req == nil {
		return errors.New("nil request")
	}
	if len(req.Logs) == 0 && len(req.Metrics) == 0 && len(req.Spans) == 0 {
		return errors.New("empty payload")
	}
	defaultSvc := req.ResourceAttrs["service.name"]
	if defaultSvc == "" {
		defaultSvc = "unknown"
	}
	for _, l := range req.Logs {
		if l.Service == "" {
			return errors.New("log record missing service")
		}
	}
	for _, m := range req.Metrics {
		if m.Service == "" || m.Name == "" {
			return errors.New("metric point missing service or name")
		}
	}
	for _, s := range req.Spans {
		if s.Service == "" || s.TraceID == "" || s.SpanID == "" {
			return errors.New("span missing service or trace_id")
		}
	}
	_ = defaultSvc
	return nil
}

// Normalize applies resource attributes (e.g. service.name) to records that
// do not have their own service field. It also sets sensible defaults for
// severity and timestamp so downstream code doesnt need to deal with zeros.
//
// Tenant scoping: if the request carries a TenantID, that value is copied
// down to every record so the store and query layer can partition data
// per tenant. If a record already has its own TenantID (rare) it wins.
func (in *Ingestor) Normalize(req *model.OTLPRequest) model.OTLPRequest {
	out := model.OTLPRequest{
		TenantID:      req.TenantID,
		ResourceAttrs: req.ResourceAttrs,
		Logs:          append([]model.LogRecord(nil), req.Logs...),
		Metrics:       append([]model.MetricPoint(nil), req.Metrics...),
		Spans:         append([]model.SpanRecord(nil), req.Spans...),
	}
	defaultSvc := req.ResourceAttrs["service.name"]
	if defaultSvc == "" {
		defaultSvc = "unknown"
	}
	now := time.Now()
	for i := range out.Logs {
		if out.Logs[i].TenantID == "" {
			out.Logs[i].TenantID = out.TenantID
		}
		if out.Logs[i].Service == "" {
			out.Logs[i].Service = defaultSvc
		}
		if out.Logs[i].Severity == "" {
			out.Logs[i].Severity = model.SeverityInfo
		}
		if out.Logs[i].Timestamp.IsZero() {
			out.Logs[i].Timestamp = now
		}
	}
	for i := range out.Metrics {
		if out.Metrics[i].TenantID == "" {
			out.Metrics[i].TenantID = out.TenantID
		}
		if out.Metrics[i].Service == "" {
			out.Metrics[i].Service = defaultSvc
		}
		if out.Metrics[i].Name == "" {
			out.Metrics[i].Name = "unknown"
		}
		if out.Metrics[i].Type == "" {
			out.Metrics[i].Type = "gauge"
		}
		if out.Metrics[i].Timestamp.IsZero() {
			out.Metrics[i].Timestamp = now
		}
	}
	for i := range out.Spans {
		if out.Spans[i].TenantID == "" {
			out.Spans[i].TenantID = out.TenantID
		}
		if out.Spans[i].Service == "" {
			out.Spans[i].Service = defaultSvc
		}
		if out.Spans[i].Status == "" {
			out.Spans[i].Status = "unset"
		}
		if out.Spans[i].StartTime.IsZero() {
			out.Spans[i].StartTime = now
		}
	}
	return out
}

// Submit enqueues a payload for asynchronous write. Returns false if the
// ingestor queue is full (backpressure).
func (in *Ingestor) Submit(req model.OTLPRequest) bool {
	in.recentMu.Lock()
	in.recent = append(in.recent, req)
	if len(in.recent) > 64 {
		in.recent = in.recent[len(in.recent)-64:]
	}
	in.recentMu.Unlock()

	return in.pool.Submit(batch.Job{
		Payload: req,
		Fn: func(ctx context.Context, payload any) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			r := payload.(model.OTLPRequest)
			in.store.InsertLogs(r.Logs)
			in.store.InsertMetrics(r.Metrics)
			in.store.InsertSpans(r.Spans)
			return nil
		},
	})
}

// SubmitSync performs a write synchronously (good for tests / admin APIs).
func (in *Ingestor) SubmitSync(req model.OTLPRequest) model.OTLPResponse {
	in.store.InsertLogs(req.Logs)
	in.store.InsertMetrics(req.Metrics)
	in.store.InsertSpans(req.Spans)
	return model.OTLPResponse{
		AcceptedLogs:    len(req.Logs),
		AcceptedMetrics: len(req.Metrics),
		AcceptedSpans:   len(req.Spans),
	}
}

// RecentPayloads returns a snapshot of the last seen payloads (for demos).
func (in *Ingestor) RecentPayloads() []model.OTLPRequest {
	in.recentMu.RLock()
	defer in.recentMu.RUnlock()
	out := make([]model.OTLPRequest, len(in.recent))
	copy(out, in.recent)
	return out
}
