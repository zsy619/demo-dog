// Package dualwal 双写 WAL：写入双份日志以提高持久性。
package dualwal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"sync"
)

// WAL is a dual-writer append-only log. Each record has a
// 4-byte length + 4-byte CRC + payload.
type WAL struct {
	mu      sync.Mutex
	primary io.Writer
	mirror  io.Writer
	records uint64
}

// ErrBadRecord is returned when the WAL detects a corrupt
// frame on read.
var ErrBadRecord = errors.New("bad record")

// New creates a dual-writer WAL. primary is required; mirror
// may be nil.
func New(primary, mirror io.Writer) *WAL {
	return &WAL{primary: primary, mirror: mirror}
}

// Append writes a record to both writers.
func (w *WAL) Append(p []byte) error {
	if len(p) > 0xFFFFFFFF {
		return errors.New("record too large")
	}
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint32(hdr[0:4], uint32(len(p)))
	crc := crc32.ChecksumIEEE(p)
	binary.BigEndian.PutUint32(hdr[4:8], crc)
	if _, err := w.primary.Write(hdr); err != nil {
		return err
	}
	if _, err := w.primary.Write(p); err != nil {
		return err
	}
	if w.mirror != nil {
		if _, err := w.mirror.Write(hdr); err != nil {
			return err
		}
		if _, err := w.mirror.Write(p); err != nil {
			return err
		}
	}
	w.mu.Lock()
	w.records++
	w.mu.Unlock()
	return nil
}

// Count returns the number of records appended.
func (w *WAL) Count() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.records
}

// Reader reads records from a WAL stream.
type Reader struct {
	r io.Reader
}

// NewReader creates a Reader from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// Next reads the next record. io.EOF on end.
func (r *Reader) Next() ([]byte, error) {
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(r.r, hdr); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, io.EOF
		}
		return nil, err
	}
	length := binary.BigEndian.Uint32(hdr[0:4])
	stored := binary.BigEndian.Uint32(hdr[4:8])
	buf := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r.r, buf); err != nil {
			return nil, err
		}
	}
	if crc32.ChecksumIEEE(buf) != stored {
		return nil, ErrBadRecord
	}
	return buf, nil
}

// Verify reads all records from r and confirms checksum.
// Returns the count verified.
func (r *Reader) Verify() (int, error) {
	n := 0
	for {
		if _, err := r.Next(); err != nil {
			if err == io.EOF {
				return n, nil
			}
			return n, err
		}
		n++
	}
}
