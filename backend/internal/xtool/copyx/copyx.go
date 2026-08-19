// Package copyx 提供基于 encoding/gob 的深拷贝辅助。
package copyx

import (
	"bytes"
	"encoding/gob"
)

// Deep 通过 gob 对 v 进行深拷贝。
// 要求 v 是可 gob 编码（不包含 chan/func/interface）。
func Deep[T any](v T) (T, error) {
	var zero T
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(&v); err != nil {
		return zero, err
	}
	var out T
	dec := gob.NewDecoder(&buf)
	if err := dec.Decode(&out); err != nil {
		return zero, err
	}
	return out, nil
}

// MustDeep 同 Deep，失败返回零值。
func MustDeep[T any](v T) T {
	out, _ := Deep(v)
	return out
}
