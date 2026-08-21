package alerts

// view.go:RecordingStateView 与其 Format 方法。
//
// RecordingStateView 是规则的 JSON 稳定结构,供 API 暴露。
// Format 返回 Prometheus /rules 兼容的条目。

import (
	"fmt"
	"time"
)

// RecordingStateView 是单条规则状态的 JSON 稳定形式。
type RecordingStateView struct {
	Name        string            `json:"name"`                  // 规则名
	NewMetric   string            `json:"new_metric"`            // 输出指标名
	Description string            `json:"description,omitempty"` // 描述
	Interval    time.Duration     `json:"interval_ns"`           // 周期(纳秒)
	Labels      map[string]string `json:"labels,omitempty"`      // 输出标签
	LastValue   float64           `json:"last_value"`            // 最近值
	LastAt      time.Time         `json:"last_at,omitempty"`     // 最近评估时间
	LastError   string            `json:"last_error,omitempty"`  // 最近错误
	Runs        int64             `json:"runs"`                  // 累计运行次数
	Fails       int64             `json:"fails"`                 // 累计失败次数
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
