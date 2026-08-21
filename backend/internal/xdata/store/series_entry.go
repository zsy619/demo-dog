package store

// series_entry.go:SeriesEntry + MetricCard 类型定义。
//
// SeriesEntry: 某指标名下的一个唯一标签集。
// MetricCard: 一个指标名的汇总信息。

// SeriesEntry 是某指标观察到的一个唯一标签集。
type SeriesEntry struct {
	Service string            `json:"service"`            // 服务名(从 labels 推断)
	Name    string            `json:"name"`               // 指标名
	Labels  map[string]string `json:"labels,omitempty"`   // 完整标签集
	LastMs  int64             `json:"last_ms"`            // 最近一次采样时间
}

// MetricCard 汇总一个指标名称。
type MetricCard struct {
	Name        string `json:"name"`                  // 指标名
	Series      int    `json:"series"`                // 不同标签集数量
	Samples     int    `json:"samples"`               // 累计采样数
	Services    int    `json:"services"`              // 不同 service 数
	FirstSeenMs int64  `json:"first_seen_ms,omitempty"` // 首次采样时间
	LastSeenMs  int64  `json:"last_seen_ms,omitempty"`  // 最近采样时间
}
