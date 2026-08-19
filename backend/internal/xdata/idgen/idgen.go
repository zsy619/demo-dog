// Package idgen 提供多种 ID 生成策略：自增、随机、雪花（简化版）。
package idgen

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

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

// RandomGenerator 是基于 crypto/rand 的随机 ID 生成器。
type RandomGenerator struct{}

// NewRandom 创建一个随机 ID 生成器。
func NewRandom() *RandomGenerator { return &RandomGenerator{} }

// Hex 返回 n 字节随机十六进制字符串。
func (g *RandomGenerator) Hex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// B64 返回 n 字节随机 base64 字符串。
func (g *RandomGenerator) B64(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Snowflake 是一个简化版雪花 ID。
type Snowflake struct {
	mu     sync.Mutex
	epoch  int64
	node   int64
	seq    int64
	lastMs int64
}

// NewSnowflake 创建一个 node 区分的雪花 ID。
func NewSnowflake(node int64) *Snowflake {
	return &Snowflake{
		epoch: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		node:  node & 0x3FF,
	}
}

// Next 返回下一个雪花 ID。
func (s *Snowflake) Next() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	if now == s.lastMs {
		s.seq = (s.seq + 1) & 0xFFF
		if s.seq == 0 {
			for now <= s.lastMs {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.seq = 0
	}
	s.lastMs = now
	ts := now - s.epoch
	return (ts << 22) | (s.node << 12) | s.seq
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
	return strings.Repeat("x", 0) + string(b[:])
}
