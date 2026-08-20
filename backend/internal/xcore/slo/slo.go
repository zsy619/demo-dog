// Package slo SLO 追踪：记录成功/失败计数与烧毁率。
//
// 本包按类型拆分到多个文件：
//   - slo.go    SLO 定义与 Sample 输入类型
//   - status.go Status 输出类型
//   - tracker.go Tracker 主体与所有公共方法
//   - internal.go 私有辅助（percentile / ratioBoundedBy）
package slo

import (
	"errors"
	"time"
)

// SLO 表示一项服务等级目标。
type SLO struct {
	Name        string        // SLO 名称（用于 Register / Record / Compute）
	Description string        // 人类可读描述
	Target      float64       // 目标成功率，范围 (0, 1]；0.999 表示 99.9%
	Window      time.Duration // 评估窗口（事件超过此时间则被淘汰）
}

// Validate 对 SLO 执行完整性检查。
func (s *SLO) Validate() error {
	if s.Name == "" {
		return errors.New("name required")
	}
	if s.Target <= 0 || s.Target > 1 {
		return errors.New("target must be in (0,1]")
	}
	if s.Window <= 0 {
		return errors.New("window must be positive")
	}
	return nil
}

// Sample 表示一次观测记录。
//
// Success == true 表示请求成功；Took 是请求耗时。
type Sample struct {
	Success bool          // 请求是否成功
	Took    time.Duration // 请求耗时
	At      time.Time     // 采样时间（由 Tracker.Record 内部填充）
}
