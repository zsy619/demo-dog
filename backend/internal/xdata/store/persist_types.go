package store

// persist_types.go:PersistSnapshot + PersistHistogram 类型定义。

import (
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// PersistSnapshot 是 Doris 引擎的可序列化表示。
//
// 刻意只保留 hot tier + service summary + MV bucket,使 cold tier
// 可在下次接入时重建。
//
// Round 30 增加 Histograms(OTel 分桶聚合 + t-digest 中心),
// 以便百分位状态能跨重启保留。
type PersistSnapshot struct {
	Version    int                              // 快照格式版本
	SavedAt    time.Time                        // 写入时间
	HotLogs    []model.LogRecord                // hot 日志记录
	HotMetrics map[string][]model.MetricPoint   // hot 指标点(按 key)
	HotSpans   map[string][]model.SpanRecord    // hot span 记录
	MV1m       map[string][]model.MVBucket      // 1 分钟 MV
	MV5m       map[string][]model.MVBucket      // 5 分钟 MV
	Services   map[string]*model.ServiceSummary // 服务摘要
	Histograms map[string]*PersistHistogram      // 直方图聚合
}

// PersistHistogram 是 histogramAgg 的磁盘形式。
type PersistHistogram struct {
	Bounds    []float64           // 分桶边界
	Counts    []int64             // 各桶计数
	Sum       float64             // 窗口内总和
	Total     int64               // 样本总数
	Min       float64             // 最小值
	Max       float64             // 最大值
	Centroids []CentroidSnapshot  // t-digest 中心
	TDTotal   int64               // t-digest 样本总数
	TDMin     float64             // t-digest 最小值
	TDMax     float64             // t-digest 最大值
}
