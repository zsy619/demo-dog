package store

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// WAL is a tiny append-only log that records every insert operation
// as a length-prefixed gob blob. On startup the snapshot restores the
// in-memory state and the WAL is replayed to bring the engine up to
// date without losing the last few seconds of writes between
// checkpoints.
//
// The format is intentionally dead simple:
//
//	header  8 bytes magic 0xD06
//         4 bytes version (currently 1)
//         4 bytes op code (1=log, 2=metric, 3=span)
//         4 bytes length of the gob payload
//         N bytes of gob-encoded model.{LogRecord,MetricPoint,SpanRecord}
//
// Reads skip unknown op codes (forward compat) and corrupt frames are
// truncated at the end of the file so the next open truncates them.
//
// The WAL is fsynced on every Append. That is enough for demo-dog
// because the bottleneck is the in-memory hot tier; the WAL only needs
// to survive crashes between snapshots. Round 23.4 can swap in batched
// fsync if write amplification becomes a problem.
type WAL struct {
	mu sync.Mutex
	f  *os.File
	en *bufio.Writer
}

const walMagic uint32 = 0xD06
const walVersion uint32 = 1

const (
	opLog    uint32 = 1
	opMetric uint32 = 2
	opSpan   uint32 = 3
)

// OpenWAL opens (or creates) the WAL file at path. The file is
// truncated to the last good record on open so a partial write at the
// end is silently discarded.
func OpenWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	if err := repairWAL(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &WAL{f: f, en: bufio.NewWriter(f)}, nil
}

func repairWAL(f *os.File) error {
	// Walk the file from the start, locating the last truncated frame
	// (if any). We do this in a single linear pass; for the demo
	// workload a WAL rarely exceeds a few MB between snapshots.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r := bufio.NewReader(f)
	var good int64
	for {
		off, _ := f.Seek(0, io.SeekCurrent)
		var hdr [16]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return err
		}
		magic := binary.BigEndian.Uint32(hdr[0:4])
		if magic != walMagic {
			// Corrupt frame: stop here and truncate.
			break
		}
		_ = binary.BigEndian.Uint32(hdr[4:8])  // version
		length := binary.BigEndian.Uint32(hdr[12:16])
		if length > 16<<20 {
			// A 16 MiB single record is nonsense for our payloads;
			// treat as corruption.
			break
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return err
		}
		good = off + 16 + int64(length)
	}
	if good > 0 {
		if _, err := f.Seek(good, io.SeekStart); err != nil {
			return err
		}
	} else {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}
	return f.Truncate(good)
}

// Append records one entry to the WAL. The entry is fsynced before
// the call returns so a crash can lose at most the last batch that
// was in-flight in the ingest pool.
func (w *WAL) Append(op uint32, payload any) error {
	buf, err := encodeGob(payload)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	hdr := [16]byte{}
	binary.BigEndian.PutUint32(hdr[0:4], walMagic)
	binary.BigEndian.PutUint32(hdr[4:8], walVersion)
	binary.BigEndian.PutUint32(hdr[8:12], op)
	binary.BigEndian.PutUint32(hdr[12:16], uint32(len(buf)))
	if _, err := w.en.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.en.Write(buf); err != nil {
		return err
	}
	if err := w.en.Flush(); err != nil {
		return err
	}
	return w.f.Sync()
}

// Close flushes and closes the underlying file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.en != nil {
		_ = w.en.Flush()
	}
	if w.f != nil {
		return w.f.Close()
	}
	return nil
}

// Rotate truncates the WAL. Call after a successful snapshot to bound
// the replay window.
func (w *WAL) Rotate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.en.Flush(); err != nil {
		return err
	}
	if err := w.f.Truncate(0); err != nil {
		return err
	}
	_, err := w.f.Seek(0, io.SeekStart)
	return err
}

// Replay drains the WAL into a fresh Doris. The function is exposed
// for tests; production code uses the unexported replayInto.
func (w *WAL) Replay() ([]model.LogRecord, []model.MetricPoint, []model.SpanRecord, error) {
	w.mu.Lock()
	path := ""
	if w.f != nil {
		path = w.f.Name()
		_ = w.en.Flush()
	}
	w.mu.Unlock()
	// Open a fresh read-only fd because O_APPEND + Seek is not
	// guaranteed to behave predictably on every OS.
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, nil, nil, err
	}
	r := bufio.NewReader(io.LimitReader(f, stat.Size()))
	var logs []model.LogRecord
	var metrics []model.MetricPoint
	var spans []model.SpanRecord
	for {
		var hdr [16]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, nil, nil, err
		}
		magic := binary.BigEndian.Uint32(hdr[0:4])
		if magic != walMagic {
			break
		}
		op := binary.BigEndian.Uint32(hdr[8:12])
		length := binary.BigEndian.Uint32(hdr[12:16])
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, nil, nil, err
		}
		switch op {
		case opLog:
			var l []model.LogRecord
			if err := decodeGob(body, &l); err == nil {
				logs = append(logs, l...)
			}
		case opMetric:
			var m []model.MetricPoint
			if err := decodeGob(body, &m); err == nil {
				metrics = append(metrics, m...)
			}
		case opSpan:
			var s []model.SpanRecord
			if err := decodeGob(body, &s); err == nil {
				spans = append(spans, s...)
			}
		}
	}
	return logs, metrics, spans, nil
}

func encodeGob(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeGob(b []byte, v any) error {
	dec := gob.NewDecoder(bytes.NewReader(b))
	return dec.Decode(v)
}

// SnapshotThenWAL is the orchestration helper for the persist loop.
// Save a snapshot, then rotate the WAL so the next replay only
// contains records added since this snapshot.
func SnapshotThenWAL(d *Doris, snapPath string, w *WAL) error {
	if err := d.SaveToFile(snapPath); err != nil {
		return fmt.Errorf("snapshot save: %w", err)
	}
	if w != nil {
		if err := w.Rotate(); err != nil {
			return fmt.Errorf("wal rotate: %w", err)
		}
	}
	return nil
}

// PeriodicPersist runs SnapshotThenWAL on the given interval until
// the returned cancel function is called. Useful from main.go:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	go store.PeriodicPersist(ctx, d, snapPath, w, 5*time.Minute)
func PeriodicPersist(interval time.Duration, d *Doris, snapPath string, w *WAL) {
	t := time.NewTicker(interval)
	for range t.C {
		_ = SnapshotThenWAL(d, snapPath, w)
	}
}
