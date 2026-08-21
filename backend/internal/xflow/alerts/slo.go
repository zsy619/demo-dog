// Package alerts SLO 错误预算计算器。
//
// An SLO is a target availability: e.g. "99% of requests succeed
// over a rolling 30-day window". The error budget is the
// percentage of allowed failures: 1 - 0.99 = 1% of all requests.
// Burn rate is the ratio of current failure rate to the budget
// rate; a burn rate of 2x means we're consuming the budget twice
// as fast as allowed.
//
// 本包提供以下计算:
//
//   - Budget left(% of the budget not yet consumed)
//   - 滑动窗口 burn rate
//   - 多窗口 burn-rate 分类(基于 Google SRE 工作手册)
//     page on (5min,1h) >= 14.4x AND
//     (30min,6h) >= 6x within the same error budget
//
// The engine 读取 from the data store via a CountSink so it
// can plug into logs, metrics, or spans. Real wiring is in
// cmd/dog-collector; this package is the pure calculator.
//
// 文件职责拆分:
//   - slo.go       包文档 + SLO 类型与校验
//   - budget.go    CountSink 接口 + BudgetStatus + Compute
//   - burn.go      BurnRate + BurnRates
//   - decide.go    MultiWindowDecision + Decide + Score
package alerts

import (
	"errors"
	"time"
)

// SLO 定义一个服务级目标。
type SLO struct {
	Name         string        // SLO 名称
	Service      string        // 所属服务
	Target       float64       // 0.99 = 99% 目标可用率
	Window       time.Duration // 评估窗口
	TotalCounter string        // 总请求数计数器名
	BadCounter   string        // 失败请求数计数器名
}

// Validate 在 SLO 不可用时报告错误。
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
