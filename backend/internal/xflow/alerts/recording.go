// 录制规则引擎。
//
// A recording rule is a named, precomputed query. It runs against
// the live data set on every evaluation cycle and stores the
// series under a new metric name. Use cases:
//
//   * Pre-aggregate: sum(rate(http_requests[5m])) -> http_requests_5m
//   * Window rollups: avg_over_time(latency[1h]) -> latency_hourly
//   * Stable SLO inputs: compute error_budget_burn() once per cycle
//
// Recording rules live alongside alert rules in /api/v1/rules and
// appear under data.groups[].rules[] with type=recording.
// Package alerts 告警规则引擎：评估规则并触发告警事件。
package alerts

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RecordingResult 是一次录制规则评估。
type RecordingResult struct {
	Metric string
	Labels map[string]string
	Value  float64
	At     time.Time
	Took   time.Duration
	Err    error
}

// RecordingRule 定义一条录制规则。
type RecordingRule struct {
	Name        string
	NewMetric   string
	Description string
	Interval    time.Duration
	Labels      map[string]string
	Evaluate    func(ctx context.Context) (float64, error)
}

// RecordingEngine 在共享 goroutine 上运行录制规则。
type RecordingEngine struct {
	mu     sync.RWMutex
	rules  map[string]*recordingState
	Persist func(ctx context.Context, r RecordingResult)
	stopCh chan struct{}
}

type recordingState struct {
	rule    RecordingRule
	lastErr error
	lastAt  time.Time
	lastVal float64
	runs    int64
	fails   int64
}

// NewRecordingEngine 返回无规则的引擎。
func NewRecordingEngine() *RecordingEngine {
	return &RecordingEngine{rules: make(map[string]*recordingState), stopCh: make(chan struct{})}
}

// Add 注册一条规则。如果已存在则返回旧规则。
func (e *RecordingEngine) Add(r RecordingRule) *RecordingRule {
	if r.Interval <= 0 {
		r.Interval = 30 * time.Second
	}
	if r.NewMetric == "" {
		r.NewMetric = r.Name
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	old, ok := e.rules[r.Name]
	e.rules[r.Name] = &recordingState{rule: r}
	if ok {
		return &old.rule
	}
	return nil
}

// Remove 注销一条规则。若存在则返回 true。
func (e *RecordingEngine) Remove(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.rules[name]
	delete(e.rules, name)
	return ok
}

// Start 为每条规则启动一个 goroutine。
func (e *RecordingEngine) Start(ctx context.Context) {
	go e.loop(ctx)
}

// Stop 关闭所有规则 goroutine。
func (e *RecordingEngine) Stop() {
	select {
	case <-e.stopCh:
	default:
		close(e.stopCh)
	}
}

func (e *RecordingEngine) loop(ctx context.Context) {
	tickers := map[string]*time.Ticker{}
	cancels := map[string]chan struct{}{}
	for {
		select {
		case <-ctx.Done():
			for _, c := range cancels {
				close(c)
			}
			return
		case <-e.stopCh:
			for _, c := range cancels {
				close(c)
			}
			return
		default:
		}
		e.mu.Lock()
		for name, st := range e.rules {
			if _, exists := tickers[name]; exists {
				continue
			}
			t := time.NewTicker(st.rule.Interval)
			tickers[name] = t
			c := make(chan struct{})
			cancels[name] = c
			go e.runRule(ctx, st, t.C, c)
		}
		e.mu.Unlock()
		time.Sleep(time.Second)
	}
}

func (e *RecordingEngine) runRule(ctx context.Context, st *recordingState, tick <-chan time.Time, cancel chan struct{}) {
	for {
		select {
		case <-cancel:
			return
		case <-ctx.Done():
			return
		case <-tick:
			start := time.Now()
			res := RecordingResult{Metric: st.rule.NewMetric, Labels: st.rule.Labels, At: start}
			v, err := st.rule.Evaluate(ctx)
			res.Value = v
			res.Took = time.Since(start)
			res.Err = err
			e.mu.Lock()
			st.runs++
			if err != nil {
				st.fails++
				st.lastErr = err
			} else {
				st.lastErr = nil
				st.lastVal = v
				st.lastAt = start
			}
			e.mu.Unlock()
			if e.Persist != nil && err == nil {
				e.Persist(ctx, res)
			}
		}
	}
}

// Snapshot 返回所有规则的运行时状态。
func (e *RecordingEngine) Snapshot() []RecordingStateView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]RecordingStateView, 0, len(e.rules))
	for _, st := range e.rules {
		out = append(out, RecordingStateView{
			Name:        st.rule.Name,
			NewMetric:   st.rule.NewMetric,
			Description: st.rule.Description,
			Interval:    st.rule.Interval,
			Labels:      st.rule.Labels,
			LastValue:   st.lastVal,
			LastAt:      st.lastAt,
			LastError:   errString(st.lastErr),
			Runs:        st.runs,
			Fails:       st.fails,
		})
	}
	return out
}

// RecordingStateView 是单条规则状态的 JSON 稳定形式。
type RecordingStateView struct {
	Name        string            `json:"name"`
	NewMetric   string            `json:"new_metric"`
	Description string            `json:"description,omitempty"`
	Interval    time.Duration     `json:"interval_ns"`
	Labels      map[string]string `json:"labels,omitempty"`
	LastValue   float64           `json:"last_value"`
	LastAt      time.Time         `json:"last_at,omitempty"`
	LastError   string            `json:"last_error,omitempty"`
	Runs        int64             `json:"runs"`
	Fails       int64             `json:"fails"`
}

func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

// Format 返回 Prometheus 兼容的规则条目。
func (v RecordingStateView) Format() map[string]any {
	return map[string]any{
		"name":       v.Name,
		"query":      fmt.Sprintf("recording_rule(%s)", v.NewMetric),
		"type":       "recording",
		"labels":     v.Labels,
		"value":      v.LastValue,
		"interval":   v.Interval.String(),
		"runs":       v.Runs,
		"fails":      v.Fails,
		"last_eval":  v.LastAt,
		"last_error": v.LastError,
	}
}
