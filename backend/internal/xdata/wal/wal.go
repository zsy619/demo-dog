package wal

// wal.go:WAL 主体。
//
// WAL 是仅追加的预写日志,支持周期性快照压缩。
// 使用 sync.Mutex 保护写操作与 snap / hasSnap 字段。

import (
	"bufio"
	"os"
	"sync"
)

// WAL 是支持定期快照的仅追加日志。
type WAL struct {
	mu      sync.Mutex   // 保护写并发
	path    string       // 磁盘路径
	file    *os.File     // 文件句柄
	writer  *bufio.Writer // 缓冲写入器
	snap    *Snapshot    // 最近一次快照
	hasSnap bool         // 是否存在快照
}

// Open 在给定路径打开(或创建)WAL。
func Open(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w := &WAL{path: path, file: f}
	w.writer = bufio.NewWriter(f)
	return w, nil
}

// Close 刷新并关闭文件。
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

// Append 向 WAL 写入一条记录。
func (w *WAL) Append(seq uint64, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	frame := encodeFrame(seq, payload)
	if _, err := w.writer.Write(frame); err != nil {
		return err
	}
	return w.writer.Flush()
}

// WriteSnapshot 持久化快照负载并截断 seq <= snap.Seq 的旧条目。
//
// 内部调用 compactLocked 完成截断。
func (w *WAL) WriteSnapshot(seq uint64, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.snap = &Snapshot{Seq: seq, Payload: payload}
	w.hasSnap = true
	return w.compactLocked()
}

// LastSnapshot 返回内存中的最近快照(深拷贝);不存在时返回 nil。
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

// compactLocked 重写 WAL,只保留 snap.Seq 之后的条目。
//
// 必须持有 w.mu 时调用。
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
	// 将快照重写为合成条目。
	snapFrame := encodeFrame(0, encodeSnapshotBlob(w.snap))
	if _, err := w.writer.Write(snapFrame); err != nil {
		return err
	}
	return w.writer.Flush()
}
