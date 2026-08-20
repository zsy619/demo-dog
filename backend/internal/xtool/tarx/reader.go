// Package tarx reader.go: 流式 tar 读取器。
package tarx

import (
	"bytes"
	"errors"
	"io"
)

// Entry 是 Reader.Next 返回的一个 tar 条目。
type Entry struct {
	Header *Header
	Body   io.Reader
}

// Reader 按流式方式遍历一个 tar 归档。
type Reader struct {
	r io.Reader
}

// NewReader 构造一个 Reader。
func NewReader(r io.Reader) *Reader { return &Reader{r: r} }

// Next 返回下一个条目;到末尾时返回 (nil, io.EOF)。
//
// 解析失败但条目存在时仍返回条目 + 错误(bad header / bad checksum)。
func (r *Reader) Next() (*Entry, error) {
	hdr := make([]byte, headerSize)
	if _, err := io.ReadFull(r.r, hdr); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	if isZeroBlock(hdr) {
		return nil, io.EOF
	}
	h, err := ParseHeader(hdr)
	if err != nil && !errors.Is(err, ErrBadHeader) && !errors.Is(err, ErrBadChecksum) {
		return nil, err
	}
	body := io.LimitReader(r.r, h.Size)
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	round := (h.Size + 511) &^ 511
	if round > h.Size {
		if _, err := io.CopyN(io.Discard, r.r, round-h.Size); err != nil {
			return nil, err
		}
	}
	return &Entry{Header: h, Body: bytes.NewReader(buf)}, nil
}

// isZeroBlock 判断一个 512 字节块是否全为 0 (tar 末尾的两个空块)。
func isZeroBlock(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
