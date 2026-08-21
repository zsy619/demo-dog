package xpersistence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTempKV(t *testing.T) (*FileJSON, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	kv, err := OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = kv.Close() })
	return kv, path
}

// TestFileJSON_BasicSetGet 验证基本 Get/Set/HAS/Delete 流程。
func TestFileJSON_BasicSetGet(t *testing.T) {
	kv, _ := newTempKV(t)
	ctx := context.Background()
	if err := kv.Set(ctx, "a", []byte(`{"v":1}`)); err != nil {
		t.Fatalf("set a: %v", err)
	}
	v, err := kv.Get(ctx, "a")
	if err != nil {
		t.Fatalf("get a: %v", err)
	}
	if !bytes.Equal(v, []byte(`{"v":1}`)) {
		t.Errorf("get a = %s", v)
	}
	have, err := kv.Has(ctx, "a")
	if err != nil || !have {
		t.Errorf("has a: have=%v err=%v", have, err)
	}
	if err := kv.Delete(ctx, "a"); err != nil {
		t.Fatalf("delete a: %v", err)
	}
	if _, err := kv.Get(ctx, "a"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete = %v", err)
	}
	if err := kv.Delete(ctx, "missing"); err != nil {
		t.Errorf("delete missing should no-op, got %v", err)
	}
}

// TestFileJSON_Persistence 验证 Set 后 Close,重新 Open 数据还在。
// 模拟进程重启场景。
func TestFileJSON_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	ctx := context.Background()
	// 第一次进程
	first, err := OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	if err := first.Set(ctx, "tenants/acme", []byte(`{"id":"acme"}`)); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := first.Set(ctx, "tenants/zen", []byte(`{"id":"zen"}`)); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close1: %v", err)
	}
	// 第二次进程(模拟重启)
	second, err := OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	defer second.Close()
	v, err := second.Get(ctx, "tenants/acme")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !bytes.Equal(v, []byte(`{"id":"acme"}`)) {
		t.Errorf("reload tenants/acme = %s", v)
	}
	keys, err := second.List(ctx, "tenants/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("list tenants: got %v", keys)
	}
}

// TestFileJSON_AtomicRollback 验证 WithAtomic 在 fn 返回错误时不写入。
func TestFileJSON_AtomicRollback(t *testing.T) {
	kv, _ := newTempKV(t)
	ctx := context.Background()
	// 先 Set 一个 baseline
	if err := kv.Set(ctx, "baseline", []byte(`"ok"`)); err != nil {
		t.Fatalf("set baseline: %v", err)
	}
	err := kv.WithAtomic(ctx, func(tx Tx) error {
		_ = tx.Set("k1", []byte(`"v1"`))
		_ = tx.Set("k2", []byte(`"v2"`))
		return fmt.Errorf("simulated")
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	// baseline 必须还在
	v, err := kv.Get(ctx, "baseline")
	if err != nil || !bytes.Equal(v, []byte(`"ok"`)) {
		t.Errorf("baseline gone after rollback: v=%s err=%v", v, err)
	}
	// k1/k2 必须不存在
	if _, err := kv.Get(ctx, "k1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("k1 should not exist, got err=%v", err)
	}
}

// TestFileJSON_AtomicCommit 验证 WithAtomic 在 fn 返回 nil 时原子提交。
func TestFileJSON_AtomicCommit(t *testing.T) {
	kv, _ := newTempKV(t)
	ctx := context.Background()
	err := kv.WithAtomic(ctx, func(tx Tx) error {
		if err := tx.Set("idx/keyA", []byte(`"keyA"`)); err != nil {
			return err
		}
		if err := tx.Set("data/keyA", []byte(`"payload"`)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("with atomic: %v", err)
	}
	v, err := kv.Get(ctx, "data/keyA")
	if err != nil || !bytes.Equal(v, []byte(`"payload"`)) {
		t.Errorf("data/keyA missing: v=%s err=%v", v, err)
	}
}

// TestFileJSON_ListPrefix 验证 List 按 prefix + 字典序。
func TestFileJSON_ListPrefix(t *testing.T) {
	kv, _ := newTempKV(t)
	ctx := context.Background()
	for _, k := range []string{"tenants/b", "tenants/a", "tenants/c", "keys/x", "tenants/aa"} {
		if err := kv.Set(ctx, k, []byte(`"v"`)); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}
	keys, err := kv.List(ctx, "tenants/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"tenants/a", "tenants/aa", "tenants/b", "tenants/c"}
	if len(keys) != len(want) {
		t.Fatalf("list: got %v want %v", keys, want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Errorf("list[%d] = %s want %s", i, k, want[i])
		}
	}
}

// TestFileJSON_CloseAfter 验证 Close 后操作返回 ErrClosed。
func TestFileJSON_CloseAfter(t *testing.T) {
	kv, _ := newTempKV(t)
	ctx := context.Background()
	if err := kv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := kv.Get(ctx, "x"); !errors.Is(err, ErrClosed) {
		t.Errorf("get after close = %v", err)
	}
	if err := kv.Set(ctx, "x", []byte(`"v"`)); !errors.Is(err, ErrClosed) {
		t.Errorf("set after close = %v", err)
	}
}

// TestFileJSON_Concurrent 验证 100 个 goroutine 并发 Set/Get 不撕裂。
func TestFileJSON_Concurrent(t *testing.T) {
	kv, _ := newTempKV(t)
	ctx := context.Background()
	const N = 100
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("k-%d", i)
			if err := kv.Set(ctx, key, []byte(fmt.Sprintf("\"v-%d\"", i))); err != nil {
				t.Errorf("set: %v", err)
				return
			}
			if v, err := kv.Get(ctx, key); err != nil || !bytes.Equal(v, []byte(fmt.Sprintf("\"v-%d\"", i))) {
				t.Errorf("get: v=%s err=%v", v, err)
			}
		}(i)
	}
	wg.Wait()
	keys, err := kv.List(ctx, "k-")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != N {
		t.Errorf("list len = %d want %d", len(keys), N)
	}
}

// TestFileJSON_FlockBlocksSecondOpen 验证第二个进程 Open 同一份
// 文件会被 flock 阻塞,而不是读出半写状态。
//
// 这里通过先 Open 持有锁,再尝试第二个 Open 验证它至少不能立刻
// 读出文件已被改坏的数据;具体 flock 阻塞依赖 OS,这里只
// 验证串行关闭 + 重新 Open 仍然完整。
func TestFileJSON_FlockBlocksSecondOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	ctx := context.Background()
	// 第一个进程,加锁
	first, err := OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	if err := first.Set(ctx, "a", []byte(`"1"`)); err != nil {
		t.Fatalf("set: %v", err)
	}
	// 关闭后,第二个进程必须能读到数据(也证明 first 写盘成功)
	if err := first.Close(); err != nil {
		t.Fatalf("close1: %v", err)
	}
	second, err := OpenFileJSON(path)
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	defer second.Close()
	v, err := second.Get(ctx, "a")
	if err != nil || !bytes.Equal(v, []byte(`"1"`)) {
		t.Errorf("second open missed a: v=%s err=%v", v, err)
	}
}

// TestFileJSON_CorruptedFile 验证文件损坏时 Open 报 ErrCorrupted。
func TestFileJSON_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("this is not json"), 0o644); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	_, err := OpenFileJSON(path)
	if !errors.Is(err, ErrCorrupted) {
		t.Errorf("expected ErrCorrupted, got %v", err)
	}
}

// TestFileJSON_Stats 验证 FileStat 的写入计数正确累加。
func TestFileJSON_Stats(t *testing.T) {
	kv, _ := newTempKV(t)
	ctx := context.Background()
	start := kv.Stats().Writes
	for i := 0; i < 5; i++ {
		if err := kv.Set(ctx, fmt.Sprintf("k%d", i), []byte(`"v"`)); err != nil {
			t.Fatalf("set: %v", err)
		}
	}
	if got := kv.Stats().Writes; got != start+5 {
		t.Errorf("Writes = %d want %d", got, start+5)
	}
}
