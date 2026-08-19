// Package procstate 采集并以结构化方式输出进程状态信息，
// 用于健康检查页、SLO 报告和运行时诊断。
package procstate

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Snapshot 是进程在某时刻的状态快照。
type Snapshot struct {
	CapturedAt  time.Time         `json:"captured_at"`
	GoVersion   string            `json:"go_version"`
	GOOS        string            `json:"goos"`
	GOARCH      string            `json:"goarch"`
	NumCPU      int               `json:"num_cpu"`
	NumGoroutine int             `json:"num_goroutine"`
	Mem         MemoryStats       `json:"mem"`
	Uptime      time.Duration     `json:"uptime"`
	Custom      map[string]int64  `json:"custom"`
}

// MemoryStats 包含核心运行时内存指标。
type MemoryStats struct {
	AllocBytes      uint64 `json:"alloc"`
	TotalAllocBytes uint64 `json:"total_alloc"`
	SysBytes        uint64 `json:"sys"`
	HeapObjects    uint64 `json:"heap_objects"`
	NumGC          uint32 `json:"num_gc"`
}

// Recorder 累积启动时间与自定义计数器。
type Recorder struct {
	start   time.Time
	counters sync.Map
}

// New 创建一个 Recorder。
func New() *Recorder {
	return &Recorder{start: time.Now()}
}

// IncCounter 增加名为 name 的自定义计数器。
func (r *Recorder) IncCounter(name string) {
	v, _ := r.counters.LoadOrStore(name, new(atomic.Int64))
	v.(*atomic.Int64).Add(1)
}

// SetCounter 设置一个计数器的当前值。
func (r *Recorder) SetCounter(name string, n int64) {
	v, _ := r.counters.LoadOrStore(name, new(atomic.Int64))
	v.(*atomic.Int64).Store(n)
}

// Snapshot 返回当前进程状态。
func (r *Recorder) Snapshot() Snapshot {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	custom := map[string]int64{}
	r.counters.Range(func(k, v any) bool {
		custom[k.(string)] = v.(*atomic.Int64).Load()
		return true
	})
	return Snapshot{
		CapturedAt:   time.Now(),
		GoVersion:    runtime.Version(),
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		Mem: MemoryStats{
			AllocBytes:      ms.Alloc,
			TotalAllocBytes: ms.TotalAlloc,
			SysBytes:        ms.Sys,
			HeapObjects:     ms.HeapObjects,
			NumGC:           ms.NumGC,
		},
		Uptime: time.Since(r.start),
		Custom: custom,
	}
}

// CounterNames 按字母序返回所有自定义计数器名。
func (r *Recorder) CounterNames() []string {
	out := []string{}
	r.counters.Range(func(k, _ any) bool {
		out = append(out, k.(string))
		return true
	})
	sort.Strings(out)
	return out
}

// CounterValue 返回某个计数器的当前值。
func (r *Recorder) CounterValue(name string) int64 {
	v, ok := r.counters.Load(name)
	if !ok {
		return 0
	}
	return v.(*atomic.Int64).Load()
}

// Reset 清空所有自定义计数器。
func (r *Recorder) Reset() {
	r.counters.Range(func(k, _ any) bool {
		r.counters.Delete(k)
		return true
	})
}
