package xpersistence

// filejson.go: JSON 文件后端。
//
// 数据组织为一个嵌套的 map[string]json.RawMessage 文件;
// 每次 Set/Delete 都重写整个文件 —— 这是单进程自托管
// 场景下的最简单实现,fsync 由 tmp+rename 保证原子性,
// 跨进程串行由 flock 互斥保证。
//
// 不适合:
//   - KV 数量 > 10_000(每次写要重写整个文件)
//   - 高频写 > 100/s(同上)
//
// 适合:
//   - 配置类小数据(tenant / key / oidc / retention 等)
//   - 单进程 demo-dog 实例,几千 key 之内
//
// 如果未来需要扩容,可以无感切到 bbolt / sqlite,无需改 KV
// 接口的所有调用方。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

// FileJSON 是 JSON 文件后端的 KV 实现。
//
// 内部数据结构为 map[string][]byte,key 是 byte 字符串,
// value 是任意 JSON 兼容字节串(调用方自己序列化)。
//
// 线程安全:
//   - 内存层 sync.RWMutex
//   - 文件层 flock(LOCK_EX)防止两个 demo-dog 进程同时打开
//     同一份文件导致数据损坏
type FileJSON struct {
	path string
	file *os.File // 用于 flock;也是主数据文件

	// 内存态
	mu    sync.RWMutex
	data  map[string][]byte
	stat  *FileStat // 元数据
	dirty bool      // 是否有未持久化的改动

	// 事务内缓冲:WithAtomic 期间所有 Set/Delete 写到 staging,
	// fn 返回 nil 才合并到 data 并落盘。
	inTxn    bool
	txnStage map[string][]byte
	txnDel   map[string]struct{}

	closed bool
}

// FileStat 记录一些与文件持久化本身相关的元数据,与 KV 内容无关。
// 写在主文件顶部的 "_stat" key 里。
type FileStat struct {
	Version   int       `json:"version"`   // 数据 schema 版本,目前固定 1
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Writes    uint64    `json:"writes"`     // 累计 Set/Delete 次数
}

const (
	fileJSONSchemaVersion = 1
	statKeyPrefix         = "_stat:" // FileStat 自身的 key 前缀
)

// OpenFileJSON 打开或创建 path 指向的 JSON 文件,加载现有
// 数据并加写锁。
//
// flock 失败意味着另一个进程正在写同一份文件 —— 当前
// 实现是阻塞等待,而不是失败返回。
func OpenFileJSON(path string) (*FileJSON, error) {
	if path == "" {
		return nil, errors.New("xpersistence: path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("xpersistence: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("xpersistence: open: %w", err)
	}
	// 跨进程互斥。LOCK_SH 是共享读(允许多个进程同时打开);
	// 但写时升级到 LOCK_EX。简单起见,这里直接 LOCK_EX 持锁
	// 到 Close,单进程自托管场景下没影响。
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("xpersistence: flock: %w", err)
	}
	fj := &FileJSON{
		path: path,
		file: f,
		data: make(map[string][]byte),
	}
	if err := fj.load(); err != nil {
		f.Close()
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		return nil, err
	}
	return fj, nil
}

// load 从磁盘加载 JSON 内容到内存。文件为空 / 不存在视为空 map。
func (f *FileJSON) load() error {
	fi, err := f.file.Stat()
	if err != nil {
		return fmt.Errorf("xpersistence: stat: %w", err)
	}
	if fi.Size() == 0 {
		f.stat = &FileStat{
			Version:   fileJSONSchemaVersion,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		return nil
	}
	var raw map[string]json.RawMessage
	dec := json.NewDecoder(f.file)
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("%w: %v", ErrCorrupted, err)
	}
	// 解析 stat
	if s, ok := raw[statKeyPrefix+"self"]; ok {
		var st FileStat
		if err := json.Unmarshal(s, &st); err != nil {
			return fmt.Errorf("%w: bad stat: %v", ErrCorrupted, err)
		}
		if st.Version != fileJSONSchemaVersion {
			return fmt.Errorf("%w: schema version %d != %d", ErrCorrupted, st.Version, fileJSONSchemaVersion)
		}
		f.stat = &st
	} else {
		f.stat = &FileStat{
			Version:   fileJSONSchemaVersion,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
	}
	// 解析 KV
	for k, v := range raw {
		if len(k) >= len(statKeyPrefix) && k[:len(statKeyPrefix)] == statKeyPrefix {
			continue
		}
		// 复制出来,避免 raw 的回收引用
		vcopy := make([]byte, len(v))
		copy(vcopy, v)
		f.data[k] = vcopy
	}
	return nil
}

// flush 把内存态原子写入磁盘。
// 策略:序列化整个 data + stat 到 tmp 文件,fsync,然后 rename。
// 出错时 tmp 文件会被清理。
func (f *FileJSON) flush() error {
	if !f.dirty {
		return nil
	}
	// 序列化
	out := make(map[string]json.RawMessage, len(f.data)+1)
	for k, v := range f.data {
		out[k] = json.RawMessage(v)
	}
	f.stat.UpdatedAt = time.Now().UTC()
	f.stat.Writes++
	statRaw, err := json.Marshal(f.stat)
	if err != nil {
		return fmt.Errorf("xpersistence: stat marshal: %w", err)
	}
	out[statKeyPrefix+"self"] = statRaw
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("xpersistence: encode: %w", err)
	}
	// 写到 tmp + rename
	dir := filepath.Dir(f.path)
	base := filepath.Base(f.path)
	tmp, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("xpersistence: tmp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // 任何路径出错都清掉 tmp
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("xpersistence: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("xpersistence: fsync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("xpersistence: close tmp: %w", err)
	}
	if err := os.Rename(tmpName, f.path); err != nil {
		return fmt.Errorf("xpersistence: rename: %w", err)
	}
	// 文件本身也 fsync 一遍,确保 rename 在目录项里也持久化。
	if err := f.file.Sync(); err != nil {
		return fmt.Errorf("xpersistence: fsync file: %w", err)
	}
	f.dirty = false
	return nil
}

// Get 实现 KV.Get。返回 ErrNotFound 而不是 nil bytes。
func (f *FileJSON) Get(ctx context.Context, key string) ([]byte, error) {
	if f.closed {
		return nil, ErrClosed
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.inTxn {
		if _, ok := f.txnDel[key]; ok {
			return nil, ErrNotFound
		}
		if v, ok := f.txnStage[key]; ok {
			out := make([]byte, len(v))
			copy(out, v)
			return out, nil
		}
	}
	v, ok := f.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

// Set 实现 KV.Set。设置 key 的值为 value;value 必须是合法
// JSON 字节串,否则文件无法回读。
func (f *FileJSON) Set(ctx context.Context, key string, value []byte) error {
	if f.closed {
		return ErrClosed
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inTxn {
		vcopy := make([]byte, len(value))
		copy(vcopy, value)
		f.txnStage[key] = vcopy
		delete(f.txnDel, key)
		return nil
	}
	vcopy := make([]byte, len(value))
	copy(vcopy, value)
	f.data[key] = vcopy
	f.dirty = true
	return f.flush()
}

// Delete 实现 KV.Delete。对不存在的 key 是 no-op。
func (f *FileJSON) Delete(ctx context.Context, key string) error {
	if f.closed {
		return ErrClosed
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inTxn {
		delete(f.txnStage, key)
		f.txnDel[key] = struct{}{}
		return nil
	}
	if _, ok := f.data[key]; !ok {
		return nil
	}
	delete(f.data, key)
	f.dirty = true
	return f.flush()
}

// Has 实现 KV.Has。
func (f *FileJSON) Has(ctx context.Context, key string) (bool, error) {
	if f.closed {
		return false, ErrClosed
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.inTxn {
		if _, ok := f.txnDel[key]; ok {
			return false, nil
		}
		if _, ok := f.txnStage[key]; ok {
			return true, nil
		}
	}
	_, ok := f.data[key]
	return ok, nil
}

// List 实现 KV.List。按 key 字典序返回所有以 prefix 开头的 key。
func (f *FileJSON) List(ctx context.Context, prefix string) ([]string, error) {
	if f.closed {
		return nil, ErrClosed
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]string, 0)
	for k := range f.data {
		if len(k) >= len(statKeyPrefix) && k[:len(statKeyPrefix)] == statKeyPrefix {
			continue
		}
		if prefix == "" || (len(k) >= len(prefix) && k[:len(prefix)] == prefix) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}

// WithAtomic 实现 KV.WithAtomic。
//
// 实现策略:锁住整个 KV,准备 staging 缓冲,调用 fn;
// fn 返回 nil 时把 staging 合并到 data 并 flush,否则丢弃 staging。
//
// 注意:filejson 的 "事务" 只保证原子性,不保证并发性能;
// 多个 goroutine 并发 WithAtomic 会串行化。
func (f *FileJSON) WithAtomic(ctx context.Context, fn func(Tx) error) error {
	if f.closed {
		return ErrClosed
	}
	if fn == nil {
		return errors.New("xpersistence: fn nil")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inTxn {
		return errors.New("xpersistence: nested WithAtomic not supported")
	}
	f.inTxn = true
	f.txnStage = make(map[string][]byte)
	f.txnDel = make(map[string]struct{})
	err := fn(&filejsonTx{f: f})
	f.inTxn = false
	if err != nil {
		f.txnStage = nil
		f.txnDel = nil
		return fmt.Errorf("%w: %v", ErrAtomicFn, err)
	}
	// 提交
	for k := range f.txnDel {
		delete(f.data, k)
	}
	for k, v := range f.txnStage {
		f.data[k] = v
	}
	f.txnStage = nil
	f.txnDel = nil
	f.dirty = true
	return f.flush()
}

// filejsonTx 实现 Tx 接口,操作 WithAtomic 的 staging 缓冲。
type filejsonTx struct {
	f *FileJSON
}

func (t *filejsonTx) Get(key string) ([]byte, error) {
	// 注意:WithAtomic 已经持锁,Tx 内不再加锁。
	if _, ok := t.f.txnDel[key]; ok {
		return nil, ErrNotFound
	}
	if v, ok := t.f.txnStage[key]; ok {
		out := make([]byte, len(v))
		copy(out, v)
		return out, nil
	}
	v, ok := t.f.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (t *filejsonTx) Set(key string, value []byte) error {
	// 同上,Tx 内不加锁。
	vcopy := make([]byte, len(value))
	copy(vcopy, value)
	t.f.txnStage[key] = vcopy
	delete(t.f.txnDel, key)
	return nil
}

func (t *filejsonTx) Delete(key string) error {
	delete(t.f.txnStage, key)
	t.f.txnDel[key] = struct{}{}
	return nil
}

func (t *filejsonTx) Has(key string) (bool, error) {
	if _, ok := t.f.txnDel[key]; ok {
		return false, nil
	}
	if _, ok := t.f.txnStage[key]; ok {
		return true, nil
	}
	_, ok := t.f.data[key]
	return ok, nil
}

func (t *filejsonTx) List(prefix string) ([]string, error) {
	out := make([]string, 0)
	// 合并视图:data 中非 deleted,且非 staging 覆盖
	for k := range t.f.data {
		if len(k) >= len(statKeyPrefix) && k[:len(statKeyPrefix)] == statKeyPrefix {
			continue
		}
		if _, deleted := t.f.txnDel[k]; deleted {
			continue
		}
		if prefix == "" || (len(k) >= len(prefix) && k[:len(prefix)] == prefix) {
			out = append(out, k)
		}
	}
	for k := range t.f.txnStage {
		if prefix != "" && (len(k) < len(prefix) || k[:len(prefix)] != prefix) {
			continue
		}
		// dedup
		found := false
		for _, existing := range out {
			if existing == k {
				found = true
				break
			}
		}
		if !found {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Close 实现 KV.Close。关闭文件并释放 flock。
func (f *FileJSON) Close() error {
	if f.closed {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	if f.dirty {
		// 尝试最后一次 flush,失败也不报错
		_ = f.flush()
	}
	if f.file != nil {
		_ = syscall.Flock(int(f.file.Fd()), syscall.LOCK_UN)
		f.file.Close()
	}
	return nil
}

// Stats 返回 FileStat 的拷贝。
func (f *FileJSON) Stats() FileStat {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.stat == nil {
		return FileStat{}
	}
	out := *f.stat
	return out
}
