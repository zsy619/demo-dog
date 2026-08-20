package store

import (
	"testing"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want error
	}{
		{"default", DefaultConfig(), nil},
		{"bad ttl", Config{HotLogTTL: -1, HotLogCap: 1, HotMetricCap: 1, ColdCap: 1}, errAny},
		{"bad log cap", Config{HotLogCap: 0, HotMetricCap: 1, ColdCap: 1}, errAny},
		{"bad metric cap", Config{HotLogCap: 1, HotMetricCap: 0, ColdCap: 1}, errAny},
		{"bad cold cap", Config{HotLogCap: 1, HotMetricCap: 1, ColdCap: 0}, errAny},
		{"bad card", Config{HotLogCap: 1, HotMetricCap: 1, ColdCap: 1, MaxCardinality: -1}, errAny},
		{"zero card ok", Config{HotLogCap: 1, HotMetricCap: 1, ColdCap: 1, MaxCardinality: 0}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if (err == nil) != (c.want == nil) {
				t.Fatalf("cfg=%+v err=%v want-err=%v", c.cfg, err, c.want)
			}
		})
	}
}

var errAny = stringErr("any")

type stringErr string

func (s stringErr) Error() string { return string(s) }

func TestCardinalityGate_Drops(t *testing.T) {
	d := New(Config{
		HotLogTTL: 5 * time.Minute, HotLogCap: 8, HotMetricCap: 64, ColdCap: 100, MaxCardinality: 2,
	})
	now := time.Now()
	// 2 distinct series accepted, 3rd distinct is dropped.
	accepted := 0
	for i, lbl := range []map[string]string{{"a": "1"}, {"a": "2"}, {"a": "3"}, {"a": "2"}} {
		n := d.InsertMetrics([]model.MetricPoint{
			{Service: "svc", Name: "http.req", Timestamp: now.Add(time.Duration(i) * time.Second), Value: 1, Labels: lbl},
		})
		accepted += n
	}
	if accepted != 3 {
		t.Fatalf("expected 3 accepted (2 distinct + 1 repeat), got %d", accepted)
	}
	cs := d.CardinalityStats()
	if cs.Current != 2 {
		t.Fatalf("current=%d", cs.Current)
	}
	if cs.Dropped != 1 {
		t.Fatalf("dropped=%d", cs.Dropped)
	}
}

func TestCardinalityGate_Unlimited(t *testing.T) {
	d := New(Config{
		HotLogTTL: 5 * time.Minute, HotLogCap: 8, HotMetricCap: 64, ColdCap: 100, MaxCardinality: 0,
	})
	now := time.Now()
	for i := 0; i < 50; i++ {
		d.InsertMetrics([]model.MetricPoint{
			{Service: "svc", Name: "http.req", Timestamp: now, Value: float64(i), Labels: map[string]string{"u": time.Now().String()}},
		})
	}
	cs := d.CardinalityStats()
	if cs.Dropped != 0 {
		t.Fatalf("dropped should be 0 when unlimited, got %d", cs.Dropped)
	}
}
