// Package workqueue 提供带延迟的优先级工作队列。
// Get 返回最早到期的可消费项；延迟项在到达执行时间前不会返回。
package workqueue

import (
	"container/heap"
	"sync"
	"time"
)

// Item 是工作项。
type Item struct {
	Key   string
	Value any
	At    time.Time
	index int
}

// Queue 是延迟工作队列。
type Queue struct {
	mu   sync.Mutex
	hp   []*Item
	key  map[string]*Item
	wait chan struct{}
}

// New 创建一个空队列。
func New() *Queue {
	return &Queue{
		hp:   make([]*Item, 0),
		key:  make(map[string]*Item),
		wait: make(chan struct{}, 1),
	}
}

// Add 把键 K 加上 value 加入队列；若已存在则覆盖并重置时间。
func (q *Queue) Add(k string, v any, at time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if it, ok := q.key[k]; ok {
		it.Value = v
		it.At = at
		heap.Fix(q, it.index)
	} else {
		it := &Item{Key: k, Value: v, At: at}
		heap.Push(q, it)
		q.key[k] = it
	}
	q.notify()
}

// AddAfter 把项目加入队列，at = now + d。
func (q *Queue) AddAfter(k string, v any, d time.Duration) {
	q.Add(k, v, time.Now().Add(d))
}

// Size 返回项目数（线程安全）。
func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.hp)
}

// Len 实现 heap.Interface。
func (q *Queue) Len() int { return len(q.hp) }

// Less 实现 heap.Interface。
func (q *Queue) Less(i, j int) bool { return q.hp[i].At.Before(q.hp[j].At) }

// Swap 实现 heap.Interface。
func (q *Queue) Swap(i, j int) {
	q.hp[i], q.hp[j] = q.hp[j], q.hp[i]
	q.hp[i].index = i
	q.hp[j].index = j
}

// Push 实现 heap.Interface。
func (q *Queue) Push(x any) { it := x.(*Item); it.index = len(q.hp); q.hp = append(q.hp, it) }

// Pop 实现 heap.Interface。
func (q *Queue) Pop() any {
	n := len(q.hp) - 1
	it := q.hp[n]
	q.hp[n] = nil
	it.index = -1
	q.hp = q.hp[:n]
	return it
}

// Get 取出最早到期的项；若未到期则阻塞到到期或 ctx 取消。
func (q *Queue) Get() (it *Item, ok bool) {
	for {
		q.mu.Lock()
		if len(q.hp) == 0 {
			q.mu.Unlock()
			<-q.wait
			continue
		}
		top := q.hp[0]
		if d := time.Until(top.At); d > 0 {
			timer := time.NewTimer(d)
			q.mu.Unlock()
			select {
			case <-timer.C:
				continue
			case <-q.wait:
				timer.Stop()
				continue
			}
		}
		heap.Pop(q)
		delete(q.key, top.Key)
		q.mu.Unlock()
		return top, true
	}
}

// Done 移除指定键（任务完成后调用）。
func (q *Queue) Done(k string) {
	q.mu.Lock()
	delete(q.key, k)
	q.mu.Unlock()
}

func (q *Queue) notify() {
	select {
	case q.wait <- struct{}{}:
	default:
	}
}
