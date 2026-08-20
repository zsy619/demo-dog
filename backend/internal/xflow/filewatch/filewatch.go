// Package filewatch 通过轮询提供 stdlib-only 的文件变更监听。
// 它跟踪每个文件的 size+mtime 哈希，发生变更时通过通道推送事件。
package filewatch

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// EventKind 表示变更类型。
type EventKind int

const (
	EventModified EventKind = iota
	EventDeleted
	EventCreated
)

// Event 是单次文件变更。
type Event struct {
	Path string
	Kind EventKind
	Size int64
	At   time.Time
}

// Watcher 轮询文件变更并通过 channel 派发。
type Watcher struct {
	mu       sync.Mutex
	files    map[string]fileSnap
	interval time.Duration
	stop     chan struct{}
	stopped  chan struct{}
	Events   chan Event
	started  atomic.Bool
}

type fileSnap struct {
	size  int64
	mtime int64
	exists bool
}

// New 创建一个轮询间隔为 interval 的 Watcher。
func New(interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	return &Watcher{
		files:    make(map[string]fileSnap),
		interval: interval,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		Events:   make(chan Event, 64),
	}
}

// ErrEmptyPath 在路径为空时返回。
var ErrEmptyPath = errors.New("filewatch: 路径为空")

// Watch 注册一个路径进行轮询。
func (w *Watcher) Watch(path string) error {
	if path == "" {
		return ErrEmptyPath
	}
	w.mu.Lock()
	w.files[path] = snapshotOf(path)
	w.mu.Unlock()
	return nil
}

// Unwatch 停止观察一个路径。
func (w *Watcher) Unwatch(path string) {
	w.mu.Lock()
	delete(w.files, path)
	w.mu.Unlock()
}

// Start 启动轮询循环。重复调用只生效一次。
func (w *Watcher) Start() {
	if w.started.CompareAndSwap(false, true) {
		go w.run()
	}
}

// Stop 停止轮询循环。Start 未调用时立即返回。
func (w *Watcher) Stop() {
	if !w.started.Load() {
		return
	}
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	<-w.stopped
}

func (w *Watcher) run() {
	defer close(w.stopped)
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			w.scan()
		}
	}
}

func (w *Watcher) scan() {
	w.mu.Lock()
	snapshot := make(map[string]fileSnap, len(w.files))
	for k, v := range w.files {
		snapshot[k] = v
	}
	w.mu.Unlock()
	for path, prev := range snapshot {
		cur := snapshotOf(path)
		w.mu.Lock()
		prevLocked := w.files[path]
		w.mu.Unlock()
		if !sameSnap(prevLocked, cur) {
			ev := Event{Path: path, Size: cur.size, At: time.Now()}
			switch {
			case !prevLocked.exists && cur.exists:
				ev.Kind = EventCreated
			case prevLocked.exists && !cur.exists:
				ev.Kind = EventDeleted
			default:
				ev.Kind = EventModified
			}
			w.mu.Lock()
			w.files[path] = cur
			w.mu.Unlock()
			select {
			case w.Events <- ev:
			default:
			}
		}
		_ = prev
	}
}

func snapshotOf(path string) fileSnap {
	fi, err := os.Stat(path)
	if err != nil {
		return fileSnap{exists: false}
	}
	return fileSnap{size: fi.Size(), mtime: fi.ModTime().UnixNano(), exists: true}
}

func sameSnap(a, b fileSnap) bool {
	return a.exists == b.exists && a.size == b.size && a.mtime == b.mtime
}
