// Package ingest 实现 OTLP 风格的 JSON 摄取管道。
//
// The public surface is intentionally narrow: a single Ingestor struct that
// knows how to take an OTLPRequest, validate it, batch it, and finally push
// it into the in-memory Doris engine via the 工作池.
//
// Two implementation details are worth highlighting:
//
//   - We coalesce many small per-request payloads into a single batch payload
//     inside the worker, so the per-write critical section is one slice append
//     per signal rather than many independent locks.
//   - We honor `RetryLogs/Metrics/Spans` in the response so the frontend can
//     decide whether to 重试 or degrade the UI.
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

// Ingestor 将 OTLP 解码器、工作池和 store 连接起来。
type Ingestor struct {
	store *store.Doris
	pool  *batch.Pool

	// 保留最近负载用于调试/重放。
	recentMu sync.RWMutex
	recent   []model.OTLPRequest
}

// New 以自定义池大小构造 Ingestor；默认为 8 个 worker。
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

// Close 排空工作池。
func (in *Ingestor) Close() {
	in.pool.Close()
}

// PoolStats 暴露工作池计数器（accepted、processed、
// retried, failed). The HTTP /metrics handler renders them as
// Prometheus counters so operators can see ingest back-pressure in
// real time.
func (in *Ingestor) PoolStats() batch.Stats {
	return in.pool.Stats()
}

// Validate 对 OTLPRequest 执行轻量级健全性检查。
// It does not check every field; missing 服务名 is the only hard error.
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

// Normalize 将资源属性（例如 service.name）应用于
// do not have their own service field. It also sets sensible defaults for
// severity and timestamp so downstream code doesnt need to deal with zeros.
//
// 租户 scoping: if the request carries a TenantID, that value is copied
// down to every record so the store and query layer can partition data
// per 租户. If a record already has its own TenantID (rare) it wins.
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

// Submit 将负载入队以异步写入。若则返回 false
// ingestor 队列 is full (背压).
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

// SubmitSync 同步执行写入（适合测试 / 管理 API）。
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

// RecentPayloads 返回最近看到的负载快照（用于 demo）。
func (in *Ingestor) RecentPayloads() []model.OTLPRequest {
	in.recentMu.RLock()
	defer in.recentMu.RUnlock()
	out := make([]model.OTLPRequest, len(in.recent))
	copy(out, in.recent)
	return out
}
