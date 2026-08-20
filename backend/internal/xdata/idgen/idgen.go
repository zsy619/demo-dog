// Package idgen 提供多种 ID 生成策略：自增、随机、雪花（简化版）。
//
// 雪花 ID 布局（64-bit）：
//   - 41 bit 时间戳（毫秒）
//   - 10 bit 节点 ID（0-1023）
//   - 12 bit 序列号（每毫秒 0-4095）
package idgen

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrClockBackward 在时钟回拨时返回（Snowflake）。
var ErrClockBackward = errors.New("idgen: 时钟回拨")

// ErrNodeTooLarge 在 node 超过 1023 时返回。
var ErrNodeTooLarge = errors.New("idgen: node 超出 10 bit 范围（0-1023）")

// IncGenerator 是单调递增 ID 生成器。
type IncGenerator struct {
	mu sync.Mutex
	v  uint64
}

// NewInc 创建一个从 start 开始的递增生成器。
func NewInc(start uint64) *IncGenerator { return &IncGenerator{v: start} }

// Next 返回下一个 ID。
func (g *IncGenerator) Next() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.v++
	return g.v
}

// Current 返回当前值（不递增）。
func (g *IncGenerator) Current() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.v
}

// RandomGenerator 是基于 crypto/rand 的随机 ID 生成器。
type RandomGenerator struct{}

// NewRandom 创建一个随机 ID 生成器。
func NewRandom() *RandomGenerator { return &RandomGenerator{} }

// Hex 返回 n 字节随机十六进制字符串。
// 随机源错误时 panic（不应发生）。
func (g *RandomGenerator) Hex(n int) string {
	b := make([]byte, n)
	mustRand(b)
	return hex.EncodeToString(b)
}

// B64 返回 n 字节随机 base64url 字符串。
func (g *RandomGenerator) B64(n int) string {
	b := make([]byte, n)
	mustRand(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func mustRand(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("idgen: crypto/rand 失败: %w", err))
	}
}

// Snowflake 是一个简化版雪花 ID。
type Snowflake struct {
	mu     sync.Mutex
	epoch  int64
	node   int64
	seq    int64
	lastMs int64
}

// DefaultEpoch 是默认起始时间（2024-01-01 UTC）。
var DefaultEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

// NewSnowflake 创建一个 node 区分的雪花 ID。
// node 必须在 0-1023 范围内。
func NewSnowflake(node int64) (*Snowflake, error) {
	if node < 0 || node > 0x3FF {
		return nil, ErrNodeTooLarge
	}
	return &Snowflake{
		epoch: DefaultEpoch,
		node:  node,
	}, nil
}

// NewSnowflakeMust 同 NewSnowflake，panic on error。
func NewSnowflakeMust(node int64) *Snowflake {
	s, err := NewSnowflake(node)
	if err != nil {
		panic(err)
	}
	return s
}

// Next 返回下一个雪花 ID。
// 时钟回拨时返回 ErrClockBackward；超过 epoch 限制时同样返回错误。
func (s *Snowflake) Next() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	if now < s.lastMs {
		return 0, ErrClockBackward
	}
	if now == s.lastMs {
		s.seq = (s.seq + 1) & 0xFFF
		if s.seq == 0 {
			// 序列号溢出，等待下一毫秒
			for now <= s.lastMs {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.seq = 0
	}
	s.lastMs = now
	ts := now - s.epoch
	if ts < 0 {
		return 0, ErrClockBackward
	}
	if ts > 0x1FFFFFFFFFF {
		return 0, fmt.Errorf("idgen: 时间戳溢出")
	}
	return (ts << 22) | (s.node << 12) | s.seq, nil
}

// DecodeSnowflake 解析雪花 ID。
func DecodeSnowflake(id int64, epoch time.Time) (ts time.Time, node, seq int64) {
	tsMs := (id >> 22) + epoch.UnixMilli()
	node = (id >> 12) & 0x3FF
	seq = id & 0xFFF
	ts = time.UnixMilli(tsMs)
	return
}

// ShortID 生成一个可读短 ID（不保证唯一但碰撞率低）。
func ShortID() string {
	const alphabet = "23456789abcdefghjkmnpqrstuvwxyz"
	var b [10]byte
	ts := uint32(time.Now().Unix())
	for i := 0; i < 6; i++ {
		b[i] = alphabet[ts%uint32(len(alphabet))]
		ts /= uint32(len(alphabet))
	}
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(time.Now().UnixNano()))
	for i := 0; i < 4; i++ {
		b[6+i] = alphabet[n[i]%byte(len(alphabet))]
	}
	return string(b[:]) + strings.Repeat("x", 0)
}
