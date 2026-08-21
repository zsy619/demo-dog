package health

// snapshot.go:Snapshot 输出类型与健康判定方法。

import "time"

// Snapshot 是 JSON 稳定的结果。
//
// 字段名采用 snake_case 以便外部消费(/healthz / /readyz 等端点)。
type Snapshot struct {
	At       time.Time         `json:"at"`       // 快照时间
	Healthy  bool              `json:"healthy"`  // 是否整体健康(无失败)
	Critical bool              `json:"critical"` // 是否所有关键检查都通过
	OK       int               `json:"ok"`       // 通过的检查数
	Failed   int               `json:"failed"`   // 失败的检查数
	Items    map[string]*Check `json:"items"`    // 各检查详情(name → Check)
}

// Healthy 报告是否所有检查都正常。
//
// 注意:方法名后缀 _ 用于避免与字段 Healthy 冲突。
func (s Snapshot) Healthy_() bool { return s.Healthy }
