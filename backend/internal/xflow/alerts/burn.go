package alerts

// burn.go:BurnRate 类型与多窗口 burn rate 计算。
//
// BurnRate 表示观测错误率与允许错误率之比:>1 表示预算消耗快于允许速度。
// BurnRates 计算 8 个标准窗口的 burn rate(Google SRE 工作手册)。

import (
	"errors"
	"time"
)

// BurnRate 是观测错误率与允许错误
// rate. >1 means we're consuming the budget faster than allowed.
type BurnRate struct {
	Window time.Duration // 窗口长度
	Rate   float64       // burn 率(>1 表示过快)
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
