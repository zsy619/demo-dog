// Package serialize 提供基于 binary 的二进制编解码辅助。
package serialize

import (
	"encoding/binary"
	"errors"
)

// Header 是编码数据起始的元数据。
type Header struct {
	Magic   uint32
	Version uint16
	Flags   uint16
}

// DefaultMagic 是默认魔术字。
const DefaultMagic = 0x44474F47 // "DGOG"

// ErrBadMagic 表示魔术字不匹配。
var ErrBadMagic = errors.New("serialize: 魔术字错误")

// Encode 把一个字节切片编码为带 Header 的字节序列。
func Encode(data []byte, hdr Header) []byte {
	buf := make([]byte, 8+len(data))
	binary.BigEndian.PutUint32(buf[0:4], hdr.Magic)
	binary.BigEndian.PutUint16(buf[4:6], hdr.Version)
	binary.BigEndian.PutUint16(buf[6:8], hdr.Flags)
	copy(buf[8:], data)
	return buf
}

// Decode 解析 Header 并返回数据。
func Decode(buf []byte, magic uint32) (Header, []byte, error) {
	if len(buf) < 8 {
		return Header{}, nil, errors.New("serialize: 过短")
	}
	m := binary.BigEndian.Uint32(buf[0:4])
	if m != magic {
		return Header{}, nil, ErrBadMagic
	}
	h := Header{
		Magic:   m,
		Version: binary.BigEndian.Uint16(buf[4:6]),
		Flags:   binary.BigEndian.Uint16(buf[6:8]),
	}
	return h, buf[8:], nil
}

// Uint64 把一个 uint64 编码为 8 字节。
func Uint64(v uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	return buf
}

// Uint32 把一个 uint32 编码为 4 字节。
func Uint32(v uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, v)
	return buf
}
