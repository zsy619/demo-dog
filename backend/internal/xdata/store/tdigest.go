package store

// tdigest.go:TDigest 类型 + 基础观察 / 计数接口。
//
// TDigest 是流式分位数估计器,基于 Dunning 与 Ertl 于 2019 年提出的
// t-digest 算法的极简、仅依赖标准库的实现。
//
// 无需存储每个观测值即可产生准确的分位数。
// 内存占用 O(delta),delta 是压缩参数(默认 100)。
// 对 SLO 的 p99/p95 查询已足够。
//
// 权衡:
//   - 分位数误差有界但非零;p99 误差通常小于分布质量的 1%。
//   - 质心会随时间合并;较老观测值贡献的逐元素权重更小。
//   - 内存:默认每个质心约 16 字节 * 2*delta ≈ 3.2 KiB。
//   - 单个互斥锁保证线程安全;常见路径(质心吸收)每次 O(1)。
//
// 参考:Computing Extremely Accurate Quantiles using t-Digests
// (Dunning & Ertl, 2019, https://github.com/tdunning/t-digest).

import (
	"math"
	"sync"
)

// TDigest 是一个流式分位数估计器。
type TDigest struct {
	mu        sync.Mutex // 保护全部状态
	centroids []centroid // 已压缩的质心列表
	delta     float64    // 压缩参数
	total     int64      // 累计观测数
	min       float64    // 最小观测值
	max       float64    // 最大观测值
	hasData   bool       // 是否已接收到数据
}

// centroid 是 t-digest 的内部节点。
type centroid struct {
	mean   float64 // 质心均值
	weight int64   // 质心权重(对应观测数)
}

// NewTDigest 返回一个指定压缩参数的 t-digest。
//
// Delta 控制准确度:值越大越精确,内存占用也越多。
// 推荐:50(粗略)、100(默认)、200(高精度);< 10 视为 10,> 1000 视为 1000。
func NewTDigest(delta float64) *TDigest {
	if delta < 10 {
		delta = 10
	}
	if delta > 1000 {
		delta = 1000
	}
	return &TDigest{delta: delta}
}

// Observe 添加一个观测值;并发安全。
func (t *TDigest) Observe(x float64) {
	if math.IsNaN(x) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.observeLocked(x, 1)
}

// ObserveBatch 添加 n 个值为 x 的观测值。
//
// 适用于下游管道已聚合了计数的场景
// (例如 OTel histogram 在第 i 桶中有 BucketCount 个观测值)。
func (t *TDigest) ObserveBatch(x float64, n int64) {
	if n <= 0 || math.IsNaN(x) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.observeLocked(x, n)
}

// Count 返回已摄入的观测值总数。
func (t *TDigest) Count() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

// Min 返回迄今为止观察到的最小值。
func (t *TDigest) Min() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.min
}

// Max 返回迄今为止观察到的最大值。
func (t *TDigest) Max() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.max
}
