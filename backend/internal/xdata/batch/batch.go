// Package batch 提供 KV 批量写入协调器：
// 收集一批 Put/Delete 请求，到达阈值时一次写入底层 Sink。
package batch

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Op 是批量操作类型。
type Op int

const (
	OpPut Op = iota
	OpDelete
)

// Entry 是单条批量项。
type Entry struct {
	Op    Op
	Key   string
	Value []byte
}

// Sink 是用户提供的批量写入端。
type Sink interface {
	Apply(ctx context.Context, entries []Entry) error
}

// ErrNoSink 在未配置 Sink 时返回。
var ErrNoSink = errors.New("batch: 未配置 Sink")

// Writer 收集批量请求并定期 flush。
type Writer struct {
	mu       sync.Mutex
	sink     Sink
	buf      []Entry
	cap      int
	interval time.Duration
	errMu    sync.Mutex
	lastErr  error
	wg       sync.WaitGroup
	stop     chan struct{}
	tick     *time.Ticker
}

// New 创建一个 Writer，cap 为单次最大缓冲；interval 为定时 flush 周期。
func New(sink Sink, cap int, interval time.Duration) *Writer {
	if cap <= 0 {
		cap = 64
	}
	if interval <= 0 {
		interval = time.Second
	}
	w := &Writer{
		sink:     sink,
		cap:      cap,
		interval: interval,
		buf:      make([]Entry, 0, cap),
		stop:     make(chan struct{}),
	}
	w.tick = time.NewTicker(interval)
	w.wg.Add(1)
	go w.loop()
	return w
}

// Put 入队一条 Put。
func (w *Writer) Put(key string, value []byte) {
	w.enqueue(Entry{Op: OpPut, Key: key, Value: value})
}

// Delete 入队一条 Delete。
func (w *Writer) Delete(key string) {
	w.enqueue(Entry{Op: OpDelete, Key: key})
}

func (w *Writer) enqueue(e Entry) {
	w.mu.Lock()
	w.buf = append(w.buf, e)
	full := len(w.buf) >= w.cap
	w.mu.Unlock()
	if full {
		_ = w.Flush()
	}
}

// Flush 强制触发一次写入。
func (w *Writer) Flush() error {
	w.mu.Lock()
	if len(w.buf) == 0 {
		w.mu.Unlock()
		return nil
	}
	batch := w.buf
	w.buf = make([]Entry, 0, w.cap)
	w.mu.Unlock()
	if w.sink == nil {
		return ErrNoSink
	}
	err := w.sink.Apply(context.Background(), batch)
	if err != nil {
		w.errMu.Lock()
		w.lastErr = err
		w.errMu.Unlock()
	}
	return err
}

// LastError 返回最近一次 Flush 的错误。
func (w *Writer) LastError() error {
	w.errMu.Lock()
	defer w.errMu.Unlock()
	return w.lastErr
}

// Close 停止定时器并 flush。
func (w *Writer) Close() error {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	w.tick.Stop()
	w.wg.Wait()
	return w.Flush()
}

func (w *Writer) loop() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stop:
			return
		case <-w.tick.C:
			_ = w.Flush()
		}
	}
}
