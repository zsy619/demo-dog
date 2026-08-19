// Package bloomx 提供可序列化的布隆过滤器。
package bloomx

import (
	"encoding/binary"
	"errors"
	"hash/fnv"
)

// Filter 是基于两个 FNV 哈希的可序列化布隆过滤器。
type Filter struct {
	bits  []uint64
	h     int
	size  uint64
	count uint64
}

// New 创建一个期望元素 n、误判率 fpr 的过滤器。
func New(n int, fpr float64) *Filter {
	if n <= 0 {
		n = 1024
	}
	if fpr <= 0 || fpr >= 1 {
		fpr = 0.01
	}
	m := int(-float64(n)*logOf(fpr) / (logOf(2) * logOf(2)))
	if m < 64 {
		m = 64
	}
	k := int(float64(m)/float64(n)*logOf(2) + 0.5)
	if k < 1 {
		k = 1
	}
	if k > 16 {
		k = 16
	}
	sz := (m + 63) / 64
	return &Filter{bits: make([]uint64, sz), h: k, size: uint64(m)}
}

// logOf 是利用 FNV64 实现的常量对数函数（避免 math 依赖）。
// 简化使用 math.Log.
func logOf(x float64) float64 {
	return log(x)
}

// Add 把 key 加入过滤器。
func (f *Filter) Add(key []byte) {
	f.count++
	a, b := f.hashes(key)
	for i := 0; i < f.h; i++ {
		p := (a + uint64(i)*b) % f.size
		f.bits[p/64] |= 1 << (p % 64)
	}
}

// Contains 检查 key 是否可能存在。
func (f *Filter) Contains(key []byte) bool {
	a, b := f.hashes(key)
	for i := 0; i < f.h; i++ {
		p := (a + uint64(i)*b) % f.size
		if f.bits[p/64]&(1<<(p%64)) == 0 {
			return false
		}
	}
	return true
}

func (f *Filter) hashes(k []byte) (uint64, uint64) {
	h1 := fnv.New64a()
	h1.Write(k)
	h2 := fnv.New64a()
	h2.Write([]byte("salt"))
	h2.Write(k)
	return h1.Sum64(), h2.Sum64()
}

// Bytes 把 Filter 序列化为字节切片。
func (f *Filter) Bytes() []byte {
	buf := make([]byte, 16+8*len(f.bits))
	binary.BigEndian.PutUint64(buf[0:8], f.size)
	binary.BigEndian.PutUint64(buf[8:16], uint64(f.count))
	for i, b := range f.bits {
		binary.BigEndian.PutUint64(buf[16+8*i:16+8*(i+1)], b)
	}
	// 追加 h
	return append(buf, byte(f.h))
}

// FromBytes 反序列化 Filter。
func FromBytes(b []byte) (*Filter, error) {
	if len(b) < 17 {
		return nil, errors.New("bloomx: 过短")
	}
	size := binary.BigEndian.Uint64(b[0:8])
	count := binary.BigEndian.Uint64(b[8:16])
	h := int(b[len(b)-1])
	data := b[16 : len(b)-1]
	if len(data)%8 != 0 {
		return nil, errors.New("bloomx: 位数据对齐错误")
	}
	bits := make([]uint64, len(data)/8)
	for i := range bits {
		bits[i] = binary.BigEndian.Uint64(data[8*i : 8*(i+1)])
	}
	return &Filter{bits: bits, h: h, size: size, count: count}, nil
}

// Size 返回位大小。
func (f *Filter) Size() uint64 { return f.size }

// HashCount 返回哈希数。
func (f *Filter) HashCount() int { return f.h }

// Count 返回已加入元素数。
func (f *Filter) Count() uint64 { return f.count }
// log 内部辅助：把对数运算外包。
func log(x float64) float64 { return logImpl(x) }

// logImpl 使用级数近似。
func logImpl(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// 使用连分数展开
	const Ln2 = 0.6931471805599453
	n := 0.0
	for x > 2 {
		x /= 2
		n++
	}
	for x < 0.5 {
		x *= 2
		n--
	}
	z := (x - 1) / (x + 1)
	s := z
	term := z
	z2 := z * z
	for i := 1; i < 50; i++ {
		term *= z2
		s += term / float64(2*i+1)
		if term < 1e-15 {
			break
		}
	}
	return 2*s + n*Ln2
}
