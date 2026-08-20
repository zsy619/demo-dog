// Package wal 预写日志:先写日志再修改状态,支持崩溃恢复与重放。
//
// 本包按类型拆分到多个文件:
//   - frame.go   磁盘帧编码/解码
//   - wal.go     WAL 主体与 Append/WriteSnapshot/LastSnapshot
//   - snapshot.go Snapshot 类型
//   - reader.go  Reader 迭代器
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
)

// magic 是磁盘帧的起始 4 字节标记。
var magic = [4]byte{'W', 'A', 'L', '1'}

// 磁盘上的帧格式:
//   4 bytes  magic 'WAL1'
//   4 bytes  length of payload (LE u32)
//   payload bytes
//   4 bytes  CRC32 of (magic+length+payload) (LE u32)
//
// CRC 用于检测撕裂写 / 损坏。

// encodeFrame 构造一个磁盘帧。
func encodeFrame(seq uint64, payload []byte) []byte {
	h := make([]byte, 16+len(payload))
	copy(h[:4], magic[:])
	binary.LittleEndian.PutUint32(h[4:8], uint32(len(payload)+8)) // length includes 8 bytes seq
	binary.LittleEndian.PutUint64(h[8:16], seq)
	copy(h[16:], payload)
	c := crc32.NewIEEE()
	c.Write(h[:16+len(payload)])
	cs := c.Sum32()
	var crcBytes [4]byte
	binary.LittleEndian.PutUint32(crcBytes[:], cs)
	return append(h, crcBytes[:]...)
}

// decodeFrame 解析磁盘帧,返回 (seq, payload, err)。
//
// 校验 magic 与 CRC;任一不匹配返回错误。
func decodeFrame(b []byte) (uint64, []byte, error) {
	if len(b) < 12 {
		return 0, nil, io.ErrShortBuffer
	}
	if string(b[:4]) != string(magic[:]) {
		return 0, nil, errors.New("bad magic")
	}
	length := binary.LittleEndian.Uint32(b[4:8])
	total := int(12 + length)
	if total > len(b) {
		return 0, nil, io.ErrShortBuffer
	}
	seq := binary.LittleEndian.Uint64(b[8:16])
	payload := make([]byte, length-8)
	copy(payload, b[16:16+length-8])
	got := binary.LittleEndian.Uint32(b[16+length-8 : 16+length-4])
	c := crc32.NewIEEE()
	c.Write(b[:16+length-8])
	want := c.Sum32()
	if got != want {
		return 0, nil, fmt.Errorf("crc mismatch: got %x want %x", got, want)
	}
	return seq, payload, nil
}
