// Package alerts 告警规则引擎:评估规则并触发告警事件。
//
// 本文件包含 recording rule 子模块:
//
//   - recording.go 包文档 + 录制规则类型 + RecordingEngine
//   - view.go     RecordingStateView 与 Format
//   - internal.go errString 私有辅助
//
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
package alerts

import (
	"context"
	"sync"
	"time"
)

// RecordingResult 是一次录制规则评估。
type RecordingResult struct {
	Metric string            // 新指标名
	Labels map[string]string // 标签
	Value  float64           // 当前值
	At     time.Time         // 评估时间
	Took   time.Duration     // 耗时
	Err    error             // 评估错误
}

// RecordingRule 定义一条录制规则。
type RecordingRule struct {
	Name        string                                  // 规则名
	NewMetric   string                                  // 输出指标名
	Description string                                  // 描述
	Interval    time.Duration                           // 评估周期
	Labels      map[string]string                       // 输出标签
	Evaluate    func(ctx context.Context) (float64, error) // 评估函数
}

// RecordingEngine 在共享协程上运行录制规则。
type RecordingEngine struct {
	mu     sync.RWMutex                          // 保护 rules
	rules  map[string]*recordingState             // name → 状态
	Persist func(ctx context.Context, r RecordingResult) // 结果回调
	stopCh chan struct{}                         // 停止信号
}

// recordingState 是 RecordingEngine 私有状态。
type recordingState struct {
	rule    RecordingRule // 规则副本
	lastErr error         // 最近一次错误
	lastAt  time.Time     // 最近一次评估时间
	lastVal float64       // 最近一次评估值
	runs    int64         // 累计运行次数
	fails   int64         // 累计失败次数
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

// Start 为每条规则启动一个协程。
func (e *RecordingEngine) Start(ctx context.Context) {
	go e.loop(ctx)
}

// Stop 关闭所有规则协程。
func (e *RecordingEngine) Stop() {
	select {
	case <-e.stopCh:
	default:
		close(e.stopCh)
	}
}

// loop 定期同步 ticker 与规则列表。
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

// runRule 单条规则的 ticker 循环。
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
