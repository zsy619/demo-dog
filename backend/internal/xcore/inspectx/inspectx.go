// Package inspectx 提供进程级运行时信息的轻量探针。
package inspectx

import (
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// Info 是运行时快照。
type Info struct {
	Now        time.Time `json:"now"`
	GOMAXPROCS int       `json:"gomaxprocs"`
	NumCPU     int       `json:"num_cpu"`
	GoRoutines int       `json:"goroutines"`
	AllocBytes uint64    `json:"alloc_bytes"`
	SysBytes   uint64    `json:"sys_bytes"`
	NumGC      uint32    `json:"num_gc"`
	GoVersion  string    `json:"go_version"`
	BuildInfo  string    `json:"build_info"`
}

// Probe 是一个探针：
// 每隔 interval 收集一次 Info，可用于 /debug/info 接口。
type Probe struct {
	mu     sync.RWMutex
	latest atomic.Pointer[Info]
}

// NewProbe 创建一个 Probe 并立即采集一次。
func NewProbe() *Probe {
	p := &Probe{}
	p.Refresh()
	return p
}

// Refresh 触发一次运行时采集。
func (p *Probe) Refresh() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	bi, _ := debug.ReadBuildInfo()
	build := ""
	if bi != nil {
		build = bi.GoVersion + " " + bi.Path
	}
	info := &Info{
		Now:        time.Now(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		NumCPU:     runtime.NumCPU(),
		GoRoutines: runtime.NumGoroutine(),
		AllocBytes: m.Alloc,
		SysBytes:   m.Sys,
		NumGC:      m.NumGC,
		GoVersion:  runtime.Version(),
		BuildInfo:  build,
	}
	p.latest.Store(info)
}

// Snapshot 返回最近一次的快照。
func (p *Probe) Snapshot() Info {
	v := p.latest.Load()
	if v == nil {
		return Info{Now: time.Now()}
	}
	return *v
}

// Capture 一次性采集当前状态（不依赖 Probe）。
func Capture() Info {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	bi, _ := debug.ReadBuildInfo()
	build := ""
	if bi != nil {
		build = bi.GoVersion + " " + bi.Path
	}
	return Info{
		Now:        time.Now(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		NumCPU:     runtime.NumCPU(),
		GoRoutines: runtime.NumGoroutine(),
		AllocBytes: m.Alloc,
		SysBytes:   m.Sys,
		NumGC:      m.NumGC,
		GoVersion:  runtime.Version(),
		BuildInfo:  build,
	}
}

// SetMaxProcs 设置 GOMAXPROCS（运行时并行度）。
func SetMaxProcs(n int) {
	if n < 1 {
		return
	}
	runtime.GOMAXPROCS(n)
}

// NumGoroutine 返回当前 goroutine 数。
func NumGoroutine() int { return runtime.NumGoroutine() }

// Parallel 把 items 均分到 GOMAXPROCS 个 goroutine 上并行执行。
func Parallel(items int, fn func(start, end int)) {
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if items < workers {
		workers = items
	}
	if workers < 1 {
		return
	}
	chunk := items / workers
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		start := i * chunk
		end := start + chunk
		if i == workers-1 {
			end = items
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			fn(s, e)
		}(start, end)
	}
	wg.Wait()
}
