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

// WAL 是一个极简的仅追加日志，将每次插入操作
// 记录为带长度前缀的 gob 二进制块。启动时，快照用于恢复
// 内存中的状态，然后重放 WAL 以使引擎追平最新数据，
// 避免在两次检查点之间丢失最后几秒的写入。
//
// 该格式有意做到极简：
//
//	header  8 字节 magic 0xD06
//         4 字节 version（当前为 1）
//         4 字节 op code（1=log，2=metric，3=span）
//         4 字节 gob 负载的长度
//         N 字节经 gob 编码的 model.{LogRecord,MetricPoint,SpanRecord}
//
// 读取时跳过未知的 op code（向前兼容），文件末尾的损坏帧
// 会被截断，以便下次打开时将其丢弃。
//
// WAL 在每次 Append 时都会执行 fsync。对 demo-dog 来说这已经足够，
// 因为瓶颈在内存热层级；WAL 只需在两次快照之间
// 抗住崩溃即可。第 23.4 轮可在写放大成为问题时
// 切换到批量 fsync。
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

// OpenWAL 打开（或创建）位于 path 的 WAL 文件。打开时会
// 将文件截断到最后一个完整记录处，末尾的不完整写入
// 会被静默丢弃。
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
	// 从文件头开始遍历，定位最后一个被截断的帧
	//（如有）。我们通过单次线性扫描完成此操作；
	// 对于 demo 的负载而言，两次快照之间 WAL 很少超过几 MB。
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
			// 损坏的帧：在此停止并截断。
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
