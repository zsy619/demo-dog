package store

// tdigest_snapshot.go:CentroidSnapshot + Snapshot + Restore(持久化支持)。

// CentroidSnapshot 是单个质心的可序列化形式。
type CentroidSnapshot struct {
	Mean   float64 // 质心均值
	Weight int64   // 质心权重
}

// Snapshot 返回所有质心的副本(用于持久化)。
func (t *TDigest) Snapshot() (centroids []CentroidSnapshot, total int64, min, max float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]CentroidSnapshot, len(t.centroids))
	for i, c := range t.centroids {
		out[i] = CentroidSnapshot{Mean: c.mean, Weight: c.weight}
	}
	return out, t.total, t.min, t.max
}

// Restore 从快照重建摘要。
//
// total 会被重置为传入的值(恢复后 Observe 调用的计数从该值开始递增,
// 而非从 0 开始)。
func (t *TDigest) Restore(centroids []CentroidSnapshot, total int64, min, max float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.centroids = make([]centroid, len(centroids))
	for i, c := range centroids {
		t.centroids[i] = centroid{mean: c.Mean, weight: c.Weight}
	}
	t.total = total
	t.min = min
	t.max = max
	t.hasData = total > 0
}
