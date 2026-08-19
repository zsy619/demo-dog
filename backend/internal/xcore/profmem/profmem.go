// Package profmem 提供轻量内存分析辅助：
// 读取 runtime.MemStats 关键字段，计算可用内存与分配速率。
package profmem

import (
	"runtime"
	"sync"
	"time"
)

// Snapshot 是某一时刻的内存快照。
type Snapshot struct {
	At          time.Time `json:"at"`
	AllocBytes  uint64    `json:"alloc"`
	TotalAlloc  uint64    `json:"total_alloc"`
	SysBytes    uint64    `json:"sys"`
	HeapObjects uint64    `json:"heap_objects"`
	NumGC       uint32    `json:"num_gc"`
}

// Capture 抓取当前快照。
func Capture() Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return Snapshot{
		At:          time.Now(),
		AllocBytes:  m.Alloc,
		TotalAlloc:  m.TotalAlloc,
		SysBytes:    m.Sys,
		HeapObjects: m.HeapObjects,
		NumGC:       m.NumGC,
	}
}

// Delta 计算两个快照之间的增量。
func Delta(a, b Snapshot) DeltaInfo {
	return DeltaInfo{
		Alloc: int64(b.AllocBytes - a.AllocBytes),
		Total: int64(b.TotalAlloc - a.TotalAlloc),
		Sys:   int64(b.SysBytes - a.SysBytes),
		Objs:  int64(b.HeapObjects - a.HeapObjects),
		GC:    b.NumGC - a.NumGC,
		Span:  b.At.Sub(a.At),
	}
}

// DeltaInfo 是两次快照之间的变化。
type DeltaInfo struct {
	Alloc int64         `json:"alloc_delta"`
	Total int64         `json:"total_delta"`
	Sys   int64         `json:"sys_delta"`
	Objs  int64         `json:"objects_delta"`
	GC    uint32        `json:"gc_count"`
	Span  time.Duration `json:"span"`
}

// AllocRate 返回每秒字节分配速率。
func (d DeltaInfo) AllocRate() float64 {
	if d.Span <= 0 {
		return 0
	}
	return float64(d.Total) / d.Span.Seconds()
}

// Track 持续监测分配速率。
type Track struct {
	last  Snapshot
	tick  time.Duration
	stop  chan struct{}
	mu    sync.Mutex
	Rate  float64
	On    bool
}

// NewTrack 创建一个 Track，每 tick 抓一次。
func NewTrack(tick time.Duration) *Track {
	if tick <= 0 {
		tick = time.Second
	}
	t := &Track{tick: tick, stop: make(chan struct{})}
	t.last = Capture()
	go t.run()
	return t
}

func (t *Track) run() {
	c := time.NewTicker(t.tick)
	defer c.Stop()
	for {
		select {
		case <-t.stop:
			return
		case <-c.C:
			now := Capture()
			d := Delta(t.last, now)
			t.mu.Lock()
			t.Rate = d.AllocRate()
			t.On = true
			t.last = now
			t.mu.Unlock()
		}
	}
}

// RateSnapshot 返回当前监测到的分配速率与是否已启动。
func (t *Track) RateSnapshot() (float64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Rate, t.On
}

// Stop 停止监测。
func (t *Track) Stop() {
	select {
	case <-t.stop:
	default:
		close(t.stop)
	}
}

// ForceGC 主动触发 GC。
func ForceGC() {
	runtime.GC()
}
