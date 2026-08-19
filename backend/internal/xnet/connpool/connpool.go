// Package connpool 提供一个通用连接池包装，
// 围绕 net.Conn 抽象，支持 Open/Close + Get/Put。
package connpool

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Conn 表示池中条目。
type Conn struct {
	id    int
	key   string
	open  time.Time
	close func()
}

// Pool 是一组 Conn 的集合。
type Pool struct {
	mu       sync.Mutex
	idle     map[string][]*Conn
	capacity int
	idleTo   time.Duration
	dialer   Dialer
	seq      int
}

// Dialer 是用户提供的连接创建函数。
type Dialer func(ctx context.Context) (close func(), err error)

// ErrExhausted 在获取超时时返回。
var ErrExhausted = errors.New("connpool: 无可用连接")

// New 创建一个 Pool。
func New(capacity int, idleTimeout time.Duration, dialer Dialer) *Pool {
	if capacity <= 0 {
		capacity = 8
	}
	if idleTimeout <= 0 {
		idleTimeout = 5 * time.Minute
	}
	if dialer == nil {
		dialer = func(_ context.Context) (func(), error) {
			return func() {}, nil
		}
	}
	return &Pool{
		idle:     make(map[string][]*Conn),
		capacity: capacity,
		idleTo:   idleTimeout,
		dialer:   dialer,
	}
}

// Get 按 key 获取或创建一个连接。
func (p *Pool) Get(ctx context.Context, key string) (*Conn, error) {
	p.mu.Lock()
	if idle := p.idle[key]; len(idle) > 0 {
		c := idle[len(idle)-1]
		p.idle[key] = idle[:len(idle)-1]
		p.mu.Unlock()
		return c, nil
	}
	p.mu.Unlock()
	close, err := p.dialer(ctx)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.seq++
	c := &Conn{id: p.seq, key: key, open: time.Now(), close: close}
	p.mu.Unlock()
	return c, nil
}

// Put 将连接放回池中或关闭。
func (p *Pool) Put(c *Conn) {
	if c == nil {
		return
	}
	if time.Since(c.open) > p.idleTo {
		c.close()
		return
	}
	p.mu.Lock()
	if len(p.idle[c.key]) >= p.capacity {
		p.mu.Unlock()
		c.close()
		return
	}
	p.idle[c.key] = append(p.idle[c.key], c)
	p.mu.Unlock()
}

// Close 关闭池并释放所有空闲连接。
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, list := range p.idle {
		for _, c := range list {
			c.close()
		}
	}
	p.idle = make(map[string][]*Conn)
}

// Stats 是池的统计视图。
type Stats struct {
	Idle   int            `json:"idle"`
	Total  int            `json:"total"`
	ByKey  map[string]int `json:"by_key"`
}

// Stats 返回当前统计。
func (p *Pool) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	byKey := make(map[string]int, len(p.idle))
	total := 0
	for k, v := range p.idle {
		byKey[k] = len(v)
		total += len(v)
	}
	return Stats{Idle: total, Total: total, ByKey: byKey}
}
