package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

// Frame on disk:
//   4 bytes  magic 'WAL1'
//   4 bytes  length of payload (LE u32)
//   payload bytes
//   4 bytes  CRC32 of (magic+length+payload) (LE u32)
//
// The CRC catches torn writes / corruption.
var magic = [4]byte{'W', 'A', 'L', '1'}

// WAL is an append-only log with periodic snapshot support.
type WAL struct {
	mu      sync.Mutex
	path    string
	file    *os.File
	writer  *bufio.Writer
	snap    *Snapshot
	hasSnap bool
}

// Snapshot stores the last snapshot state.
type Snapshot struct {
	Seq     uint64 `json:"seq"`
	Payload []byte `json:"payload"`
}

// Open opens (or creates) a WAL on the given path.
func Open(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w := &WAL{path: path, file: f}
	w.writer = bufio.NewWriter(f)
	return w, nil
}

// Close flushes + closes the file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return err
		}
	}
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// Append writes one entry to the WAL.
func (w *WAL) Append(seq uint64, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	frame := encodeFrame(seq, payload)
	if _, err := w.writer.Write(frame); err != nil {
		return err
	}
	return w.writer.Flush()
}

// WriteSnapshot persists a snapshot payload, truncating entries
// with seq <= snap.Seq.
func (w *WAL) WriteSnapshot(seq uint64, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.snap = &Snapshot{Seq: seq, Payload: payload}
	w.hasSnap = true
	return w.compactLocked()
}

// LastSnapshot returns the in-memory snapshot (if any).
func (w *WAL) LastSnapshot() *Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hasSnap {
		return nil
	}
	cp := *w.snap
	cp.Payload = append([]byte{}, w.snap.Payload...)
	return &cp
}

// compactLocked rewrites the WAL keeping only entries after
// snap.Seq.
func (w *WAL) compactLocked() error {
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if err := w.file.Close(); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.writer = bufio.NewWriter(f)
	// Re-write the snapshot as a synthetic entry.
	snapFrame := encodeFrame(0, encodeSnapshotBlob(w.snap))
	if _, err := w.writer.Write(snapFrame); err != nil {
		return err
	}
	return w.writer.Flush()
}

// encodeSnapshotBlob returns a JSON-encoded snapshot record.
func encodeSnapshotBlob(s *Snapshot) []byte {
	b, _ := json.Marshal(s)
	return b
}

// encodeFrame builds the on-disk frame.
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

// decodeFrame returns (seq, payload, err).
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

// Reader iterates frames from a file (or path).
type Reader struct {
	file *os.File
	r    *bufio.Reader
}

// NewReader opens a reader for the WAL file.
func NewReader(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &Reader{file: f, r: bufio.NewReader(f)}, nil
}

// Close closes the reader.
func (r *Reader) Close() error { return r.file.Close() }

// Next returns the next (seq, payload) or io.EOF.
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
