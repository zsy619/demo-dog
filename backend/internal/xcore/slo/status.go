package slo

// status.go：Status 输出类型。
//
// Status 是 JSON 稳定结构，用于监控 / admin 接口暴露当前 SLO 状态。

import "time"

// Status 表示当前 SLO 的状态。
type Status struct {
	Name        string        `json:"name"`               // SLO 名称
	Target      float64       `json:"target"`             // 目标成功率
	Window      string        `json:"window"`             // 评估窗口（字符串形式）
	Samples     int           `json:"samples"`            // 当前窗口样本总数
	Successes   int           `json:"successes"`          // 成功数
	Failures    int           `json:"failures"`           // 失败数
	Ratio       float64       `json:"ratio"`              // 当前成功率
	ErrorBudget float64       `json:"error_budget"`       // 允许的错误率（1 - target）
	Remaining   int           `json:"remaining_failures"` // 剩余允许失败次数
	Healthy     bool          `json:"healthy"`            // 当前是否达标
	P50         time.Duration `json:"p50"`                // p50 延迟
	P95         time.Duration `json:"p95"`                // p95 延迟
	P99         time.Duration `json:"p99"`                // p99 延迟
}
