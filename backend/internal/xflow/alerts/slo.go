package alerts

// SLO 错误预算计算器。
//
// An SLO is a target availability: e.g. "99% of requests succeed
// over a rolling 30-day window". The error budget is the
// percentage of allowed failures: 1 - 0.99 = 1% of all requests.
// Burn rate is the ratio of current failure rate to the budget
// rate; a burn rate of 2x means we're consuming the budget twice
// as fast as allowed.
//
// This package computes:
//
//   - Budget left (% of the budget not yet consumed).
//   - Burn rate over a sliding window.
//   - Multi-window burn-rate classification (per the Google
//     SRE workbook): page on (5min,1h) >= 14.4x AND
//     (30min,6h) >= 6x within the same error budget.
//
// The engine reads from the data store via a CountSink so it
// can plug into logs, metrics, or spans. Real wiring is in
// cmd/dog-collector; this package is the pure calculator.

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// SLO 定义一个服务级目标。
type SLO struct {
	Name         string
	Service      string
	Target       float64 // 0.99 = 99%
	Window       time.Duration
	TotalCounter string
	BadCounter   string
}

// Validate reports an error if the SLO is unusable.
func (s *SLO) Validate() error {
	if s.Name == "" {
		return errors.New("name required")
	}
	if s.Target <= 0 || s.Target >= 1 {
		return errors.New("target must be in (0, 1)")
	}
	if s.Window <= 0 {
		return errors.New("window must be positive")
	}
	return nil
}

// Budget 返回允许的失败比例。
func (s *SLO) Budget() float64 {
	return 1 - s.Target
}

// CountSink 是计数器值的唯一可信源。
type CountSink interface {
	Counter(name string, window time.Duration) int64
}

// BudgetStatus 是当前的消耗状态。
type BudgetStatus struct {
	Name              string
	Service           string
	Target            float64
	Total             int64
	Bad               int64
	ErrorRate         float64
	Budget            float64
	BudgetLeft        float64
	BudgetLeftPercent float64
	Healthy           bool
	AsOf              time.Time
}

// Compute 返回 SLO 的当前预算状态。
func Compute(s *SLO, sink CountSink, now time.Time) (BudgetStatus, error) {
	if err := s.Validate(); err != nil {
		return BudgetStatus{}, err
	}
	total := sink.Counter(s.TotalCounter, s.Window)
	bad := sink.Counter(s.BadCounter, s.Window)
	if bad > total {
		bad = total
	}
	var rate float64
	if total > 0 {
		rate = float64(bad) / float64(total)
	}
	budget := s.Budget()
	var left, leftPct float64
	if budget > 0 && rate > 0 {
		left = budget - rate
		leftPct = (left / budget) * 100
		if left < 0 {
			left = 0
			leftPct = 0
		}
	} else {
		left = budget
		leftPct = 100
	}
	healthy := left > 0
	return BudgetStatus{
		Name:              s.Name,
		Service:           s.Service,
		Target:            s.Target,
		Total:             total,
		Bad:               bad,
		ErrorRate:         rate,
		Budget:            budget,
		BudgetLeft:        left,
		BudgetLeftPercent: leftPct,
		Healthy:           healthy,
		AsOf:              now,
	}, nil
}

// BurnRate 是观测错误率与允许错误
// rate. >1 means we're consuming the budget faster than allowed.
type BurnRate struct {
	Window time.Duration
	Rate   float64
}

// BurnRates 一次计算多个窗口的 burn rate。
// Common sets per the Google SRE workbook:
//
//	5m, 30m, 1h, 2h, 6h, 1d, 3d, 7d
func BurnRates(s *SLO, sink CountSink) ([]BurnRate, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	budget := s.Budget()
	if budget <= 0 {
		return nil, errors.New("budget must be positive")
	}
	windows := []time.Duration{
		5 * time.Minute, 30 * time.Minute,
		1 * time.Hour, 2 * time.Hour, 6 * time.Hour,
		24 * time.Hour, 3 * 24 * time.Hour, 7 * 24 * time.Hour,
	}
	out := make([]BurnRate, 0, len(windows))
	for _, w := range windows {
		total := sink.Counter(s.TotalCounter, w)
		bad := sink.Counter(s.BadCounter, w)
		if bad > total {
			bad = total
		}
		var rate float64
		if total > 0 {
			rate = float64(bad) / float64(total)
		}
		out = append(out, BurnRate{Window: w, Rate: rate / budget})
	}
	return out, nil
}

// MultiWindowDecision 实现 Google SRE 工作手册
// multi-window burn-rate alert policy.
type MultiWindowDecision struct {
	ShortWindow time.Duration
	ShortBurn   float64
	LongWindow  time.Duration
	LongBurn    float64
	Level       string
	Reason      string
}

// Decide 应用多窗口策略。
func Decide(short, long BurnRate) MultiWindowDecision {
	d := MultiWindowDecision{
		ShortWindow: short.Window,
		ShortBurn:   short.Rate,
		LongWindow:  long.Window,
		LongBurn:    long.Rate,
		Level:       "none",
	}
	if short.Rate >= 14.4 && short.Window == 5*time.Minute &&
		long.Rate >= 14.4 && long.Window == 1*time.Hour {
		d.Level = "page"
		d.Reason = "fast burn: 5m/1h >= 14.4x"
		return d
	}
	if short.Rate >= 6 && short.Window == 30*time.Minute &&
		long.Rate >= 6 && long.Window == 6*time.Hour {
		d.Level = "page"
		d.Reason = "slow burn: 30m/6h >= 6x"
		return d
	}
	if short.Rate >= 3 && short.Window == 3*24*time.Hour &&
		long.Rate >= 3 && long.Window == 6*time.Hour {
		d.Level = "warn"
		d.Reason = "ticket: 6h/3d >= 3x"
		return d
	}
	if short.Rate >= 2 {
		d.Level = "warn"
		d.Reason = fmt.Sprintf("elevated: %s burn %.1fx", short.Window, short.Rate)
	}
	return d
}

// Score 返回表示健康度的单个 0..1 分数。1.0 =
// healthy, 0.0 = budget fully exhausted. Uses a logistic curve
// so the score degrades smoothly as budget left goes to zero.
func Score(status BudgetStatus) float64 {
	if status.Budget <= 0 {
		return 1
	}
	x := status.BudgetLeft / status.Budget
	if x < 0 {
		x = 0
	}
	if x > 1 {
		x = 1
	}
	z := -12 * (x - 0.5)
	return 1 / (1 + math.Exp(z))
}
