package wal

// reader.go:WAL 帧迭代器。

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Reader 从文件(或路径)迭代 WAL 帧。
type Reader struct {
	file *os.File     // 文件句柄
	r    *bufio.Reader // 缓冲读取器
}

// NewReader 打开一个 WAL 文件的 Reader。
func NewReader(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &Reader{file: f, r: bufio.NewReader(f)}, nil
}

// Close 关闭底层文件。
func (r *Reader) Close() error { return r.file.Close() }

// Next 返回下一条 (seq, payload);到末尾返回 io.EOF。
//
// 限制最大单帧 16 MiB;超出认为数据损坏。
func (r *Reader) Next() (uint64, []byte, error) {
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(r.r, hdr); err != nil {
		return 0, nil, err
	}
	length := binary.LittleEndian.Uint32(hdr[4:8])
	if int(length) < 8 || int(length) > 16*1024*1024 {
		return 0, nil, fmt.Errorf("bad length %d", length)
	}
	body := make([]byte, length+4)
	if _, err := io.ReadFull(r.r, body); err != nil {
		return 0, nil, err
	}
	frame := append(append([]byte{}, hdr...), body...)
	return decodeFrame(frame)
}
