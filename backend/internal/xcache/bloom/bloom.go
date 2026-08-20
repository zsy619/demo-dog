// Package bloom 布隆过滤器：判断一个 key 是否可能在集合中（可能有误报，无漏报）。
package bloom

import (
	"encoding/binary"
	"hash/fnv"
	"math"
)

// Filter 是一种基于布隆过滤器的概率性成员判定结构。
// 可能产生误报，但绝不会漏报。
type Filter struct {
	bits  []byte
	size  uint64
	hashN uint64
	count uint64
}

// New 返回一个 Filter，容量可容纳 n 个元素，误报率为 fpRate。
func New(n uint64, fpRate float64) *Filter {
	if n == 0 {
		n = 1
	}
	if fpRate <= 0 || fpRate >= 1 {
		fpRate = 0.01
	}
	m := optimalM(n, fpRate)
	k := optimalK(n, m)
	if k < 1 {
		k = 1
	}
	bytes := (m + 7) / 8
	return &Filter{
		bits:  make([]byte, bytes),
		size:  m,
		hashN: k,
	}
}

// Add 添加一个元素。
func (f *Filter) Add(item []byte) {
	for i := uint64(0); i < f.hashN; i++ {
		pos := hashAt(item, i) % f.size
		f.bits[pos/8] |= 1 << (pos % 8)
	}
	f.count++
}

// Contains 报告元素是否可能存在。可能产生误报。
func (f *Filter) Contains(item []byte) bool {
	for i := uint64(0); i < f.hashN; i++ {
		pos := hashAt(item, i) % f.size
		if f.bits[pos/8]&(1<<(pos%8)) == 0 {
			return false
		}
	}
	return true
}

// Count 返回已添加的元素数量。
func (f *Filter) Count() uint64 { return f.count }

// Size 返回位大小。
func (f *Filter) Size() uint64 { return f.size }

// HashN 返回每个元素的哈希次数。
func (f *Filter) HashN() uint64 { return f.hashN }

// EstimatedFPRate 返回基于当前元素数量与位大小估算出的误报率。
func (f *Filter) EstimatedFPRate() float64 {
	if f.size == 0 || f.count == 0 {
		return 0
	}
	x := float64(f.count) * float64(f.hashN) / float64(f.size)
	return math.Pow(1.0-math.Exp(-x), float64(f.hashN))
}

func hashAt(item []byte, i uint64) uint64 {
	h := fnv.New64a()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], i)
	h.Write(b[:])
	h.Write(item)
	return h.Sum64()
}

func optimalM(n uint64, fp float64) uint64 {
	m := -float64(n) * math.Log(fp) / (math.Ln2 * math.Ln2)
	if m < 1 {
		return 1
	}
	return uint64(math.Ceil(m))
}

func optimalK(n, m uint64) uint64 {
	k := float64(m) / float64(n) * math.Ln2
	if k < 1 {
		return 1
	}
	return uint64(math.Ceil(k))
}
