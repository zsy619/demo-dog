package alerts

// budget.go:CountSink 接口、BudgetStatus 结构与 Compute 计算函数。
//
// CountSink 是所有计算器的数据来源抽象,Compute 从中拉取
// TotalCounter/BadCounter 两个时间序列并返回 BudgetStatus 快照。

import "time"

// CountSink 是计数器值的唯一可信源。
type CountSink interface {
	Counter(name string, window time.Duration) int64
}

// BudgetStatus 是当前的消耗状态。
type BudgetStatus struct {
	Name              string    // SLO 名
	Service           string    // 服务名
	Target            float64   // 目标可用率
	Total             int64     // 总请求数
	Bad               int64     // 失败请求数
	ErrorRate         float64   // 实际错误率
	Budget            float64   // 允许的错误率
	BudgetLeft        float64   // 剩余预算
	BudgetLeftPercent float64   // 剩余百分比
	Healthy           bool      // 仍有预算(true = 健康)
	AsOf              time.Time // 计算时间
}

// Compute 返回 SLO 的当前预算状态。
//
// bad > total 时会被截断为 total,避免负数;
// 当 budget 与 rate 都不为 0 时计算 left / leftPct,否则视为 100%。
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
