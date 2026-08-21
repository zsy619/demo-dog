// Package alerts 实现一个轻量级规则引擎:加载一组
// alert rules + 它们的 SLO 目标,在每次 flush 时基于当前引擎状态评估,
// 当 SLO 烧毁 error budget 速度超过阈值时触发 webhook。
//
// 引擎零外部依赖(仅标准库)。Webhook 是小型 JSON envelope 的 POST;
// 接收方可以基于它再做扇出。
//
// 文件职责拆分:
//   - engine_types.go 类型定义(Severity/Rule/Fire/Provider)
//   - engine.go        Engine 主体 + 规则 CRUD
//   - evaluate.go      Evaluate + webhook 投递
package alerts

import "time"

// Severity 是告警等级。
type Severity string

const (
	SeverityInfo     Severity = "info"     // 提示
	SeverityWarning  Severity = "warning"  // 警告
	SeverityCritical Severity = "critical" // 严重
)

// Rule 描述一条 SLO burn-rate 告警规则。
type Rule struct {
	Name        string        `json:"name"`               // 规则名
	Description string        `json:"description,omitempty"` // 描述
	Service     string        `json:"service,omitempty"`  // 目标服务
	Target      float64       `json:"target"`             // 成功率目标 (0..1)
	Window      time.Duration `json:"window"`             // 慢窗窗口
	FastWindow  time.Duration `json:"fast_window"`        // 快窗窗口
	FastBurn    float64       `json:"fast_burn"`          // 快窗 burn 阈值
	SlowBurn    float64       `json:"slow_burn"`          // 慢窗 burn 阈值
	Severity    Severity      `json:"severity"`           // 告警等级
	Channels    []string      `json:"channels"`           // webhook URLs
}

// Fire 描述一次告警触发事件。
type Fire struct {
	Rule      Rule      `json:"rule"`        // 触发的规则
	Severity  Severity  `json:"severity"`    // 告警等级
	Timestamp time.Time `json:"timestamp"`   // 触发时间
	Window    string    `json:"window"`      // "fast" 或 "slow"
	Burn      float64   `json:"burn_rate"`   // burn 速率
	Reason    string    `json:"reason"`      // 触发原因说明
}

// Provider 提供评估所需的成功率数据。
//
// 返回 (ratio, n) — ratio 为窗口内成功率,n 为窗口内样本数。
type Provider interface {
	SuccessRatio(service string, window time.Duration) (ratio float64, n int)
}
