// Internal SDK statistics.
//
// The SDK keeps a few counters that are useful for debugging and
// monitoring the telemetry pipeline itself. They are cheap to update
// (single atomic increments) and safe to read at any time.
package otlp

import (
	"sync/atomic"
)

// Stats are the running counters maintained by the SDK. Read via
// SDK.Stats(). All fields are cumulative since SDK construction.
type Stats struct {
	LogsEmitted     atomic.Int64
	MetricsEmitted  atomic.Int64
	SpansEmitted    atomic.Int64

	FlushCalls      atomic.Int64
	FlushErrors     atomic.Int64
	RequeuedLogs    atomic.Int64
	RequeuedMetrics atomic.Int64
	RequeuedSpans   atomic.Int64

	DroppedLogs     atomic.Int64
	DroppedMetrics  atomic.Int64
	DroppedSpans    atomic.Int64

	SamplerSkipped  atomic.Int64
}

// Snapshot returns the current Stats as a plain struct so callers can
// marshal it (e.g. to a JSON debug endpoint) without atomic noise.
type StatsSnapshot struct {
	LogsEmitted     int64 `json:"logs_emitted"`
	MetricsEmitted  int64 `json:"metrics_emitted"`
	SpansEmitted    int64 `json:"spans_emitted"`

	FlushCalls      int64 `json:"flush_calls"`
	FlushErrors     int64 `json:"flush_errors"`
	RequeuedLogs    int64 `json:"requeued_logs"`
	RequeuedMetrics int64 `json:"requeued_metrics"`
	RequeuedSpans   int64 `json:"requeued_spans"`

	DroppedLogs     int64 `json:"dropped_logs"`
	DroppedMetrics  int64 `json:"dropped_metrics"`
	DroppedSpans    int64 `json:"dropped_spans"`

	SamplerSkipped  int64 `json:"sampler_skipped"`
}

// Snapshot returns a plain copy of the current stats.
func (s *Stats) Snapshot() StatsSnapshot {
	return StatsSnapshot{
		LogsEmitted:     s.LogsEmitted.Load(),
		MetricsEmitted:  s.MetricsEmitted.Load(),
		SpansEmitted:    s.SpansEmitted.Load(),
		FlushCalls:      s.FlushCalls.Load(),
		FlushErrors:     s.FlushErrors.Load(),
		RequeuedLogs:    s.RequeuedLogs.Load(),
		RequeuedMetrics: s.RequeuedMetrics.Load(),
		RequeuedSpans:   s.RequeuedSpans.Load(),
		DroppedLogs:     s.DroppedLogs.Load(),
		DroppedMetrics:  s.DroppedMetrics.Load(),
		DroppedSpans:    s.DroppedSpans.Load(),
		SamplerSkipped:  s.SamplerSkipped.Load(),
	}
}

// Stats returns the SDK stats snapshot. Safe for concurrent use.
func (s *SDK) Stats() StatsSnapshot {
	return (&s.stats).Snapshot()
}

// LogEmit increments the logs counter (used internally).
func (s *SDK) logEmit() { s.stats.LogsEmitted.Add(1) }

// MetricEmit increments the metrics counter (used internally).
func (s *SDK) metricEmit() { s.stats.MetricsEmitted.Add(1) }

// SpanEmit increments the spans counter (used internally).
func (s *SDK) spanEmit() { s.stats.SpansEmitted.Add(1) }
