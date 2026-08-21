package store

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// PersistSnapshot 是 Doris 引擎的可序列化表示。
// It is intentionally limited to the hot tier + service summaries + 物化视图
// buckets so cold tiers can be rebuilt on next ingest.
//
// Round 30 adds Histograms (OTel bucket aggregates + t-digest
// centroids) so 百分位 state survives restart.
type PersistSnapshot struct {
	Version    int
	SavedAt    time.Time
	HotLogs    []model.LogRecord
	HotMetrics map[string][]model.MetricPoint
	HotSpans   map[string][]model.SpanRecord
	MV1m       map[string][]model.MVBucket
	MV5m       map[string][]model.MVBucket
	Services   map[string]*model.ServiceSummary
	Histograms map[string]*PersistHistogram
}

// PersistHistogram 是 histogramAgg 的磁盘形式。
type PersistHistogram struct {
	Bounds  []float64
	Counts  []int64
	Sum     float64
	Total   int64
	Min     float64
	Max     float64
	Centroids []CentroidSnapshot
	TDTotal    int64
	TDMin      float64
	TDMax      float64
}

const persistVersion = 2

var persistOnce sync.Once

func persistRegister() {
	persistOnce.Do(func() {
		gob.Register([]model.MVBucket(nil))
		gob.Register([]model.LogRecord(nil))
		gob.Register(map[string][]model.SpanRecord(nil))
		gob.Register(map[string]*model.ServiceSummary(nil))
		gob.Register(map[string][]model.MVBucket(nil))
		gob.Register(&model.MVBucket{})
		gob.Register(&model.ServiceSummary{})
		gob.Register([]CentroidSnapshot(nil))
		gob.Register(&PersistHistogram{})
		gob.Register(map[string]*PersistHistogram(nil))
	})
}

// PersistSnapshotBytes 以 gob 编码字节返回快照。
func (d *Doris) PersistSnapshotBytes() ([]byte, error) {
	persistRegister()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	d.muLogs.RLock()
	hotLogs := append([]model.LogRecord(nil), d.hotLogs...)
	d.muLogs.RUnlock()

	d.muMetrics.RLock()
	hotMetrics := make(map[string][]model.MetricPoint, len(d.hotMetrics))
	for k, v := range d.hotMetrics {
		hotMetrics[k] = append([]model.MetricPoint(nil), v...)
	}
	d.muMetrics.RUnlock()

	d.muHistograms.RLock()
	histograms := make(map[string]*PersistHistogram, len(d.histograms))
	for k, v := range d.histograms {
		centroids, total, min, max := v.td.Snapshot()
		histograms[k] = &PersistHistogram{
			Bounds:    append([]float64(nil), v.bounds...),
			Counts:    append([]int64(nil), v.counts...),
			Sum:       v.sum,
			Total:     v.total,
			Min:       v.min,
			Max:       v.max,
			Centroids: centroids,
			TDTotal:   total,
			TDMin:     min,
			TDMax:     max,
		}
	}
	d.muHistograms.RUnlock()

	d.muSpans.RLock()
	hotSpans := make(map[string][]model.SpanRecord, len(d.hotSpans))
	for k, v := range d.hotSpans {
		hotSpans[k] = append([]model.SpanRecord(nil), v...)
	}
	d.muSpans.RUnlock()

	d.muMV.RLock()
	mv1m := copyPersistMV(d.mvMinute)
	mv5m := copyPersistMV(d.mvFiveMinute)
	d.muMV.RUnlock()

	d.muSum.RLock()
	services := make(map[string]*model.ServiceSummary, len(d.sum))
	for k, v := range d.sum {
		cp := *v
		services[k] = &cp
	}
	d.muSum.RUnlock()

	snap := PersistSnapshot{
		Version:    persistVersion,
		SavedAt:    time.Now(),
		HotLogs:    hotLogs,
		HotMetrics: hotMetrics,
		HotSpans:   hotSpans,
		MV1m:       mv1m,
		MV5m:       mv5m,
		Services:   services,
		Histograms: histograms,
	}
	if err := enc.Encode(&snap); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RestoreSnapshot 从 r 加载快照到引擎。
func (d *Doris) RestoreSnapshot(r io.Reader) error {
	persistRegister()
	dec := gob.NewDecoder(r)
	var snap PersistSnapshot
	if err := dec.Decode(&snap); err != nil {
		return err
	}
	if snap.Version != persistVersion {
		return errors.New("snapshot version mismatch")
	}

	d.muLogs.Lock()
	d.hotLogs = snap.HotLogs
	d.muLogs.Unlock()

	d.muMetrics.Lock()
	d.hotMetrics = snap.HotMetrics
	d.muMetrics.Unlock()

	d.muSpans.Lock()
	d.hotSpans = snap.HotSpans
	d.muSpans.Unlock()

	d.muMV.Lock()
	d.mvMinute = snap.MV1m
	d.mvFiveMinute = snap.MV5m
	d.muMV.Unlock()

	d.muSum.Lock()
	d.sum = snap.Services
	d.muSum.Unlock()

	if len(snap.Histograms) > 0 {
		d.muHistograms.Lock()
		d.histograms = make(map[string]*histogramAgg, len(snap.Histograms))
		for k, h := range snap.Histograms {
			agg := &histogramAgg{
				bounds:  h.Bounds,
				counts:  h.Counts,
				sum:     h.Sum,
				total:   h.Total,
				min:     h.Min,
				max:     h.Max,
				hasData: h.Total > 0,
				td:      NewTDigest(100),
			}
			agg.td.Restore(h.Centroids, h.TDTotal, h.TDMin, h.TDMax)
			d.histograms[k] = agg
		}
		d.muHistograms.Unlock()
	}
	return nil
}

// SaveToFile 原子地写入快照。
func (d *Doris) SaveToFile(path string) error {
	data, err := d.PersistSnapshotBytes()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadFromFile 加载快照。文件不存在不视为错误。
func (d *Doris) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	return d.RestoreSnapshot(f)
}

func copyPersistMV(src map[string][]model.MVBucket) map[string][]model.MVBucket {
	if src == nil {
		return nil
	}
	out := make(map[string][]model.MVBucket, len(src))
	for k, v := range src {
		out[k] = append([]model.MVBucket(nil), v...)
	}
	return out
}
