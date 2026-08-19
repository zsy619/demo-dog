// Package base58x 提供 Base58 编码与解码。
package base58x

import (
	"bytes"
	"errors"
	"math/big"
)

const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// ErrInvalid 出现在非法输入时。
var ErrInvalid = errors.New("base58x: 非法输入")

// Encode 把字节切片编码为 Base58 字符串。
func Encode(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	n := new(big.Int).SetBytes(b)
	base := big.NewInt(int64(len(alphabet)))
	zero := big.NewInt(0)
	var out []byte
	for n.Cmp(zero) > 0 {
		mod := new(big.Int)
		n.QuoRem(n, base, mod)
		out = append([]byte{alphabet[mod.Int64()]}, out...)
	}
	for _, v := range b {
		if v != 0 {
			break
		}
		out = append([]byte{alphabet[0]}, out...)
	}
	return string(out)
}

// Decode 解码 Base58 字符串为字节切片。
func Decode(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	n := new(big.Int)
	base := big.NewInt(int64(len(alphabet)))
	idx := map[byte]int64{}
	for i := 0; i < len(alphabet); i++ {
		idx[alphabet[i]] = int64(i)
	}
	leadingZeros := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == alphabet[0] {
			leadingZeros++
			continue
		}
		break
	}
	for i := leadingZeros; i < len(s); i++ {
		c := s[i]
		v, ok := idx[c]
		if !ok {
			return nil, ErrInvalid
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(v))
	}
	out := n.Bytes()
	if len(out) == 0 && leadingZeros > 0 {
		return bytes.Repeat([]byte{0}, leadingZeros), nil
	}
	return append(bytes.Repeat([]byte{0}, leadingZeros), out...), nil
}
