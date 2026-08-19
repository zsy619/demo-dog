// Package bloom 提供一个零依赖布隆过滤器。
// 使用 FNV 多哈希模拟多个独立哈希函数。
package bloom

import (
	"hash/fnv"
	"math"
	"sync"
)

// Filter 是布隆过滤器。
type Filter struct {
	mu   sync.Mutex
	bits []uint64
	h    int // 哈希函数数
	size uint64
}

// New 构造一个预期存 n、误判率 fpr 的布隆过滤器。
func New(n int, fpr float64) *Filter {
	if n <= 0 {
		n = 1024
	}
	if fpr <= 0 || fpr >= 1 {
		fpr = 0.01
	}
	// m = -n*ln(p) / (ln 2)^2
	m := int(-float64(n)*fprLn(fpr) / (fprLn(2) * fprLn(2)))
	if m < 64 {
		m = 64
	}
	// k = (m/n) * ln 2
	k := int(float64(m)/float64(n)*fprLn(2) + 0.5)
	if k < 1 {
		k = 1
	}
	sz := (m + 63) / 64
	return &Filter{bits: make([]uint64, sz), h: k, size: uint64(m)}
}

func fprLn(x float64) float64 { return math.Log(x) }

func (f *Filter) hash(seed int, k []byte) uint64 {
	h := fnv.New64a()
	h.Write([]byte{byte(seed)})
	h.Write(k)
	return h.Sum64()
}

// Add 加入一个 key。
func (f *Filter) Add(k []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i < f.h; i++ {
		p := f.hash(i, k) % f.size
		f.bits[p/64] |= 1 << (p % 64)
	}
}

// Contains 检查 key 是否可能存在。
func (f *Filter) Contains(k []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i < f.h; i++ {
		p := f.hash(i, k) % f.size
		if f.bits[p/64]&(1<<(p%64)) == 0 {
			return false
		}
	}
	return true
}

// Reset 清空所有位。
func (f *Filter) Reset() {
	f.mu.Lock()
	for i := range f.bits {
		f.bits[i] = 0
	}
	f.mu.Unlock()
}

// Size 返回位图大小。
func (f *Filter) Size() uint64 { return f.size }

// HashCount 返回哈希函数数。
func (f *Filter) HashCount() int { return f.h }
