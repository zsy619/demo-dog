// Package timernx 实现简单的时间轮 (Timing Wheel)：
// 在 slot 粒度上调度延迟任务，适用于大规模定时任务。
package timernx

import (
	"container/list"
	"sync"
	"time"
)

// Wheel 是单层时间轮。
type Wheel struct {
	mu      sync.Mutex
	slots   []*list.List
	nSlots  int
	tick    time.Duration
	curTick int64
	taskSeq int
	stop    chan struct{}
}

type item struct {
	expire int64
	fn     func()
}

// New 创建一个 tick 间隔、nSlots 个槽位的时间轮。
func New(tick time.Duration, nSlots int) *Wheel {
	if tick <= 0 {
		tick = time.Second
	}
	if nSlots <= 0 {
		nSlots = 60
	}
	w := &Wheel{
		slots:  make([]*list.List, nSlots),
		nSlots: nSlots,
		tick:   tick,
		stop:   make(chan struct{}),
	}
	for i := range w.slots {
		w.slots[i] = list.New()
	}
	go w.loop()
	return w
}

func (w *Wheel) loop() {
	t := time.NewTicker(w.tick)
	defer t.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			w.advance()
		}
	}
}

// Stop 停止时间轮。
func (w *Wheel) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
}

// Schedule 在 d 时间后执行 fn。
func (w *Wheel) Schedule(d time.Duration, fn func()) {
	if d < 0 {
		d = 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.taskSeq++
	expire := w.curTick + int64(d/w.tick) + 1
	slot := int(expire % int64(w.nSlots))
	if slot < 0 {
		slot += w.nSlots
	}
	w.slots[slot].PushBack(&item{expire: expire, fn: fn})
}

func (w *Wheel) advance() {
	w.mu.Lock()
	w.curTick++
	slot := int(w.curTick % int64(w.nSlots))
	slots := w.slots[slot]
	w.mu.Unlock()
	var fire []func()
	for e := slots.Front(); e != nil; e = e.Next() {
		it := e.Value.(*item)
		if it.expire <= w.curTick {
			fire = append(fire, it.fn)
			slots.Remove(e)
		}
	}
	for _, fn := range fire {
		func() {
			defer func() { _ = recover() }()
			fn()
		}()
	}
}
