// Package inspectx 提供进程级运行时信息的轻量探针。
package inspectx

import (
	"runtime"
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
	info := &Info{
		Now:        time.Now(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		NumCPU:     runtime.NumCPU(),
		GoRoutines: runtime.NumGoroutine(),
		AllocBytes: m.Alloc,
		SysBytes:   m.Sys,
		NumGC:      m.NumGC,
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
	return Info{
		Now:        time.Now(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		NumCPU:     runtime.NumCPU(),
		GoRoutines: runtime.NumGoroutine(),
		AllocBytes: m.Alloc,
		SysBytes:   m.Sys,
		NumGC:      m.NumGC,
	}
}
