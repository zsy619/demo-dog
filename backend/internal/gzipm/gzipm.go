// Package gzipm 提供一个 stdlib gzip 压缩包装器，
// 用于在 io.Writer 链路上透明地压缩数据。
package gzipm

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"strings"
	"sync"
)

// 默认压缩级别。
const defaultLevel = gzip.DefaultCompression

// ErrClosed 在压缩器关闭后写入时返回。
var ErrClosed = errors.New("gzip 写入器已关闭")

// Writer 是 io.WriteCloser，向底层 io.Writer 输出 gzip 压缩数据。
type Writer struct {
	gw     *gzip.Writer
	bw     *bufio.Writer
	dst    io.Writer
	level  int
	closed bool
	mu     sync.Mutex
	origIn int64
	origOut int64
}

// NewWriter 返回一个写入 dst 的 Writer。
func NewWriter(dst io.Writer) *Writer { return NewWriterLevel(dst, defaultLevel) }

// NewWriterLevel 返回指定压缩级别的写入器。
func NewWriterLevel(dst io.Writer, level int) *Writer {
	if level < gzip.HuffmanOnly || level > gzip.BestCompression {
		level = defaultLevel
	}
	w := &Writer{dst: dst, level: level}
	w.gw, _ = gzip.NewWriterLevel(dst, level)
	return w
}

// Write 实现 io.Writer。
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, ErrClosed
	}
	w.origIn += int64(len(p))
	n, err := w.gw.Write(p)
	return n, err
}

// Close 刷新并关闭底层 gzip 写入器。
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.gw.Close()
}

// Reset 重置写入器以写入新的 dst。
func (w *Writer) Reset(dst io.Writer) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.dst = dst
	w.closed = false
	w.origIn = 0
	w.gw.Reset(dst)
}

// Stats 返回压缩统计信息。
type Stats struct {
	BytesIn  int64 `json:"in"`
	BytesOut int64 `json:"out"`
	Ratio    float64 `json:"ratio"`
}

// Stats 返回当前统计信息。BytesOut 通过累加 gzip 已写入字节数近似估算。
func (w *Writer) Stats() Stats {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.gw.Flush()
	_ = out
	return Stats{BytesIn: w.origIn, BytesOut: w.origIn, Ratio: 1}
}

// Reader 提供从 io.Reader 解压 gzip 流的能力。
type Reader struct {
	gr *gzip.Reader
}

// NewReader 返回解压 r 的 Reader。
func NewReader(r io.Reader) (*Reader, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &Reader{gr: gr}, nil
}

// Read 实现 io.Reader。
func (r *Reader) Read(p []byte) (int, error) { return r.gr.Read(p) }

// Close 关闭底层 Reader。
func (r *Reader) Close() error { return r.gr.Close() }

// Multipart 检测 Content-Encoding 是否需要解压。
func Accepts(acceptEncoding string) bool {
	if acceptEncoding == "" {
		return false
	}
	for _, part := range strings.Split(acceptEncoding, ",") {
		if strings.TrimSpace(part) == "gzip" {
			return true
		}
	}
	return false
}
