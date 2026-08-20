// Package tarx writer.go: 流式 tar 写入器。
package tarx

import "io"

// Writer 按流式方式写出 tar 归档。
type Writer struct {
	w      io.Writer // 底层写入目标
	filled int64     // 已写入字节数(便于统计)
}

// NewWriter 构造一个 Writer。
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// Write 写入一个条目;body 会被完整消费。
//
// 自动填充 header 的 Size;按 512 字节对齐补零。
func (w *Writer) Write(h *Header, body []byte) error {
	if h.TypeFlag == 0 {
		h.TypeFlag = 'N'
	}
	h.Size = int64(len(body))
	hdr := BuildHeader(h)
	if _, err := w.w.Write(hdr); err != nil {
		return err
	}
	if _, err := w.w.Write(body); err != nil {
		return err
	}
	pad := (512 - (len(body) % 512)) % 512
	if pad > 0 {
		if _, err := w.w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}
	w.filled += int64(headerSize+len(body)+pad)
	return nil
}

// Close 写入两个 512 字节的全零块(tar EOF marker)。
func (w *Writer) Close() error {
	if _, err := w.w.Write(make([]byte, 1024)); err != nil {
		return err
	}
	return nil
}
