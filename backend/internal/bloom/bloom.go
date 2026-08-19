package bloom

import (
	"encoding/binary"
	"hash/fnv"
	"math"
)

// Filter is a Bloom-filter-style probabilistic membership
// test. False positives are possible; false negatives are
// not.
type Filter struct {
	bits  []byte
	size  uint64
	hashN uint64
	count uint64
}

// New returns a Filter sized to hold n items with fpRate
// false-positive probability.
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

// Add inserts an item.
func (f *Filter) Add(item []byte) {
	for i := uint64(0); i < f.hashN; i++ {
		pos := hashAt(item, i) % f.size
		f.bits[pos/8] |= 1 << (pos % 8)
	}
	f.count++
}

// Contains reports whether item may be present. False
// positives are possible.
func (f *Filter) Contains(item []byte) bool {
	for i := uint64(0); i < f.hashN; i++ {
		pos := hashAt(item, i) % f.size
		if f.bits[pos/8]&(1<<(pos%8)) == 0 {
			return false
		}
	}
	return true
}

// Count returns the number of items added.
func (f *Filter) Count() uint64 { return f.count }

// Size returns the bit size.
func (f *Filter) Size() uint64 { return f.size }

// HashN returns the number of hashes per item.
func (f *Filter) HashN() uint64 { return f.hashN }

// EstimatedFPRate returns the current estimated false
// positive rate based on count + size.
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
