package alerts

// decide.go:多窗口决策逻辑。
//
// Decide 根据短/长窗口 burn rate 决策告警级别(page / warn / none);
// Score 把 BudgetStatus 折算成 0..1 的健康分数(逻辑曲线,平滑过渡)。

import (
	"fmt"
	"math"
	"time"
)

// MultiWindowDecision 实现 Google SRE 工作手册
// multi-window burn-rate alert policy.
type MultiWindowDecision struct {
	ShortWindow time.Duration // 短窗口长度
	ShortBurn   float64       // 短窗口 burn 率
	LongWindow  time.Duration // 长窗口长度
	LongBurn    float64       // 长窗口 burn 率
	Level       string        // "page" / "warn" / "none"
	Reason      string        // 决策原因
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
