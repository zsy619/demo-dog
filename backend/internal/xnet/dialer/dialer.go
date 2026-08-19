// Package dialer 提供按 host 跟踪指数退避拨号冷却的智能拨号器。
// 当某 host 反复失败时，按指数时间窗口屏蔽再次拨号；
// 成功后清除冷却。
package dialer

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// ErrCooling 在 host 处于冷却期被屏蔽时返回。
var ErrCooling = errors.New("dialer: 冷却中")

// DialFunc 是基础拨号函数。
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Dialer 是带冷却控制的拨号器。
type Dialer struct {
	mu       sync.Mutex
	cooldown map[string]time.Time
	baseTTL  time.Duration
	maxTTL   time.Duration
	dial     DialFunc
}

// Config 是构造配置。
type Config struct {
	BaseTTL time.Duration
	MaxTTL  time.Duration
}

// New 创建一个 Dialer。
func New(c Config, d DialFunc) *Dialer {
	if c.BaseTTL <= 0 {
		c.BaseTTL = 100 * time.Millisecond
	}
	if c.MaxTTL <= 0 {
		c.MaxTTL = 30 * time.Second
	}
	if d == nil {
		d = (&net.Dialer{}).DialContext
	}
	return &Dialer{
		cooldown: make(map[string]time.Time),
		baseTTL:  c.BaseTTL,
		maxTTL:   c.MaxTTL,
		dial:     d,
	}
}

// Dial 尝试连接 addr。如果 host 处于冷却期则立即失败；
// 拨号失败时按指数提高冷却；成功则清除冷却。
func (d *Dialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if d.isCooling(host) {
		return nil, ErrCooling
	}
	conn, err := d.dial(ctx, network, addr)
	if err != nil {
		d.recordFailure(host)
		return nil, err
	}
	d.clear(host)
	return conn, nil
}

func (d *Dialer) isCooling(host string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.cooldown[host]
	if !ok {
		return false
	}
	if time.Now().After(t) {
		delete(d.cooldown, host)
		return false
	}
	return true
}

func (d *Dialer) recordFailure(host string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	cur, ok := d.cooldown[host]
	if !ok || time.Now().After(cur) {
		d.cooldown[host] = time.Now().Add(d.baseTTL)
		return
	}
	delta := cur.Sub(time.Now())
	next := delta * 2
	if next > d.maxTTL {
		next = d.maxTTL
	}
	d.cooldown[host] = time.Now().Add(next)
}

func (d *Dialer) clear(host string) {
	d.mu.Lock()
	delete(d.cooldown, host)
	d.mu.Unlock()
}

// CooldownUntil 返回 host 解封的时间。
func (d *Dialer) CooldownUntil(host string) (time.Time, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.cooldown[host]
	return t, ok
}

// Reset 清除所有冷却。
func (d *Dialer) Reset() {
	d.mu.Lock()
	d.cooldown = make(map[string]time.Time)
	d.mu.Unlock()
}
