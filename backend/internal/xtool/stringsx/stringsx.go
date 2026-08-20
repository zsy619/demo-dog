// Package stringsx 提供更高效的字符串拼接辅助。
package stringsx

import "strings"

// Join 用 sep 连接所有元素。
func Join(elems []string, sep string) string {
	return strings.Join(elems, sep)
}

// Builder 是一个简易 strings.Builder 包装，提供追加常用类型。
type Builder struct {
	b strings.Builder
}

// NewBuilder 创建空 Builder。
func NewBuilder() *Builder { return &Builder{} }

// Write 写入字符串。
func (b *Builder) Write(s string) *Builder {
	b.b.WriteString(s)
	return b
}

// WriteByte 写入字节。
func (b *Builder) WriteByte(c byte) error {
	return b.b.WriteByte(c)
}

// WriteInt 写入整数十进制。
func (b *Builder) WriteInt(v int) *Builder {
	b.b.WriteString(itoa(v))
	return b
}

// WriteByteArray 写入字节切片（hex 形式）。
func (b *Builder) WriteByteArray(a []byte) *Builder {
	const hex = "0123456789abcdef"
	for _, v := range a {
		b.b.WriteByte(hex[v>>4])
		b.b.WriteByte(hex[v&0xF])
	}
	return b
}

// String 返回拼接结果。
func (b *Builder) String() string { return b.b.String() }

// Len 返回已写入字节数。
func (b *Builder) Len() int { return b.b.Len() }

// Reset 清空。
func (b *Builder) Reset() { b.b.Reset() }

// itoa 整数转字符串。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
