// Package prioq 提供按优先级排序的最小堆以及
// 批量消费调度器。
package prioq

import (
	"container/heap"
	"sync"
	"time"
)

// Item 是堆中的一项。Index 由容器维护，不应在外部修改。
type Item struct {
	Value    any
	Priority int64
	Index    int
}

type heapImpl []*Item

func (h heapImpl) Len() int { return len(h) }
func (h heapImpl) Less(i, j int) bool { return h[i].Priority < h[j].Priority }
func (h heapImpl) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index = i
	h[j].Index = j
}
func (h *heapImpl) Push(x any) {
	it := x.(*Item)
	it.Index = len(*h)
	*h = append(*h, it)
}
func (h *heapImpl) Pop() any {
	old := *h
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	it.Index = -1
	*h = old[:n-1]
	return it
}

// Queue 是一个线程安全的最小堆优先队列。
type Queue struct {
	mu sync.Mutex
	h  heapImpl
}

func New() *Queue { return &Queue{} }

func (q *Queue) Push(v any, prio int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	heap.Push(&q.h, &Item{Value: v, Priority: prio})
}

func (q *Queue) Pop() (any, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.h.Len() == 0 {
		return nil, false
	}
	it := heap.Pop(&q.h).(*Item)
	return it.Value, true
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.h.Len()
}

// HandlerFunc 是批次处理回调。
type HandlerFunc func(items []any)

// Batch 是批处理调度器：等待 maxWait 或者积累到 maxBatch 后触发。
type Batch struct {
	q        *Queue
	maxBatch int
	maxWait  time.Duration
	closed   chan struct{}
	notify   chan struct{}
	wg       sync.WaitGroup
	handler  HandlerFunc
	total    int
	totalMu  sync.Mutex
}

func NewBatch(maxBatch int, maxWait time.Duration, h HandlerFunc) *Batch {
	if maxBatch <= 0 {
		maxBatch = 32
	}
	if maxWait <= 0 {
		maxWait = 10 * time.Millisecond
	}
	if h == nil {
		h = func(items []any) {}
	}
	b := &Batch{
		q:        New(),
		maxBatch: maxBatch,
		maxWait:  maxWait,
		closed:   make(chan struct{}),
		notify:   make(chan struct{}, 1),
		handler:  h,
	}
	b.wg.Add(1)
	go b.run()
	return b
}

func (b *Batch) Submit(v any, prio int64) {
	b.q.Push(v, prio)
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

func (b *Batch) Close() {
	close(b.closed)
	b.wg.Wait()
}

func (b *Batch) run() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.maxWait)
	defer ticker.Stop()
	for {
		select {
		case <-b.closed:
			b.flush()
			return
		case <-b.notify:
			b.drain(b.collect())
		case <-ticker.C:
			b.drain(b.collect())
		}
	}
}

func (b *Batch) collect() []any {
	out := make([]any, 0, b.maxBatch)
	for i := 0; i < b.maxBatch; i++ {
		if v, ok := b.q.Pop(); ok {
			out = append(out, v)
		} else {
			break
		}
	}
	return out
}

func (b *Batch) drain(items []any) {
	if len(items) == 0 {
		return
	}
	b.handler(items)
	b.totalMu.Lock()
	b.total += len(items)
	b.totalMu.Unlock()
}

func (b *Batch) flush() {
	for {
		items := b.collect()
		if len(items) == 0 {
			return
		}
		b.drain(items)
	}
}

func (b *Batch) Handled() int {
	b.totalMu.Lock()
	defer b.totalMu.Unlock()
	return b.total
}
