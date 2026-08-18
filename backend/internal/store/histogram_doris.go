package store

import "github.com/zsy619/demo-dog/backend/internal/model"

// updateHistograms routes histogram-type metric points into the
// (service, name) -> histogramAgg map. Non-histogram points are ignored.
func (d *Doris) updateHistograms(in []model.MetricPoint) {
	var any bool
	for _, p := range in {
		if p.Type == "histogram" && len(p.BucketBounds) > 0 {
			any = true
			break
		}
	}
	if !any {
		return
	}
	d.muHistograms.Lock()
	defer d.muHistograms.Unlock()
	for _, p := range in {
		if p.Type != "histogram" || len(p.BucketBounds) == 0 {
			continue
		}
		key := p.Service + "|" + p.Name
		h, ok := d.histograms[key]
		if !ok {
			d.histograms[key] = newHistogramAgg(p)
			continue
		}
		h.add(p)
	}
}

// HistogramSnapshot returns the aggregated histogram for one (service,
// name) pair, or nil if no histogram data has been received.
func (d *Doris) HistogramSnapshot(service, name string) *model.HistogramView {
	d.muHistograms.RLock()
	defer d.muHistograms.RUnlock()
	h, ok := d.histograms[service+"|"+name]
	if !ok {
		return nil
	}
	return h.snapshot()
}

// HistogramQuantile returns the q-th percentile of the named histogram
// (0..1). Returns 0 if the series has no data. The result is computed
// from the explicit OTel bucket boundaries so it is accurate even for
// sparse streams.
func (d *Doris) HistogramQuantile(service, name string, q float64) float64 {
	d.muHistograms.RLock()
	defer d.muHistograms.RUnlock()
	h, ok := d.histograms[service+"|"+name]
	if !ok {
		return 0
	}
	return h.quantile(q)
}
