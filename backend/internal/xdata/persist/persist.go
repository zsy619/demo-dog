// Package persist 提供基于编码 binary 的简单 KV 文件持久化。
package persist

import (
	"encoding/binary"
	"errors"
	"os"
	"sync"
)

// DB 是一个 append-only KV 存储。
type DB struct {
	mu sync.RWMutex
	f  *os.File
	m  map[string][]byte
	path string
}

// Open 打开（或创建）一个 DB 文件。
func Open(path string) (*DB, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	db := &DB{f: f, m: make(map[string][]byte), path: path}
	if err := db.load(); err != nil {
		f.Close()
		return nil, err
	}
	return db, nil
}

func (d *DB) load() error {
	st, err := d.f.Stat()
	if err != nil {
		return err
	}
	size := st.Size()
	if size == 0 {
		return nil
	}
	buf := make([]byte, size)
	if _, err := d.f.ReadAt(buf, 0); err != nil {
		return err
	}
	off := int64(0)
	for off < int64(len(buf)) {
		if off+4 > int64(len(buf)) {
			break
		}
		n := int64(binary.BigEndian.Uint32(buf[off : off+4]))
		off += 4
		if off+n > int64(len(buf)) {
			break
		}
		rec := buf[off : off+n]
		off += n
		// rec: [klen][k][vlen][v]
		if len(rec) < 4 {
			break
		}
		klen := int64(binary.BigEndian.Uint32(rec[:4]))
		if 4+klen+4 > int64(len(rec)) {
			break
		}
		k := string(rec[4 : 4+klen])
		vlen := int64(binary.BigEndian.Uint32(rec[4+klen : 4+klen+4]))
		v := rec[4+klen+4 : 4+klen+4+vlen]
		d.m[k] = v
	}
	return nil
}

// Put 设置键值。
func (d *DB) Put(k string, v []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.m[k] = v
	// append
	kb := []byte(k)
	buf := make([]byte, 4+4+len(kb)+4+len(v))
	binary.BigEndian.PutUint32(buf[0:4], uint32(4+len(kb)+4+len(v)))
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(kb)))
	copy(buf[8:8+len(kb)], kb)
	binary.BigEndian.PutUint32(buf[8+len(kb):8+len(kb)+4], uint32(len(v)))
	copy(buf[8+len(kb)+4:], v)
	if _, err := d.f.Write(buf); err != nil {
		return err
	}
	return d.f.Sync()
}

// Get 读取键值。
func (d *DB) Get(k string) ([]byte, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok := d.m[k]
	return v, ok
}

// Delete 移除一个键（仅内存；不重写文件）。
func (d *DB) Delete(k string) {
	d.mu.Lock()
	delete(d.m, k)
	d.mu.Unlock()
}

// Has 判断键是否存在。
func (d *DB) Has(k string) bool {
	d.mu.RLock()
	_, ok := d.m[k]
	d.mu.RUnlock()
	return ok
}

// Close 关闭文件。
func (d *DB) Close() error {
	return d.f.Close()
}

// Path 返回文件路径。
func (d *DB) Path() string { return d.path }

// Len 返回条目数。
func (d *DB) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.m)
}

// ErrCorrupt 表示文件已损坏。
var ErrCorrupt = errors.New("persist: 文件损坏")
