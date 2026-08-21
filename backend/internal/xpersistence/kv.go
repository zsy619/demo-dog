package xpersistence

import (
	"context"
	"errors"
)

// KV 是 demo-dog 持久化层的统一抽象。所有配置类状态
// (tenants / keys / oidc / retention / breaker / ratelimit / quota /
// webhook subscribers / alert rules) 都通过本接口落盘。
//
// 后端实现必须满足以下保证:
//   - Get 找不到 key 时返回 (nil, ErrNotFound),不要返回别的。
//   - Set 必须 fsync 才能返回成功;调用 Set 后再 Get 一定可见。
//   - Delete 对不存在的 key 是 no-op,但 err 必须为 nil。
//   - List 返回所有以 prefix 开头的 key(精确字节匹配,不是路径分量)。
//     如果 prefix 是 "" 则返回全部 key。
//   - List 的结果按 key 字典序排序;长度可能巨大,调用方负责截断。
//   - WithAtomic 跨多条 Set/Delete 提供原子性;函数返回非 nil
//     err 时所有改动回滚。
//   - Close 后任何操作返回 ErrClosed。
//
// KV 实例本身是协程安全的;多个 goroutine 可以并发调用
// 任何方法,不需要外层加锁。
type KV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	Has(ctx context.Context, key string) (bool, error)
	List(ctx context.Context, prefix string) ([]string, error)
	WithAtomic(ctx context.Context, fn func(Tx) error) error
	Close() error
}

// Tx 是 WithAtomic 的事务视图。事务内所有改动对其他
// goroutine 不可见,直到 fn 返回 nil 后才一次性提交。
//
// 事务内的 Set / Delete 不保证 fsync 立刻落盘,但 fn 返回后
// 必须保证原子提交(要么全成要么全不写)。
type Tx interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
	Has(key string) (bool, error)
	List(prefix string) ([]string, error)
}

// 持久化层的错误。所有调用方都应该通过 errors.Is 判断,
// 不要直接 == 比较。
var (
	// ErrNotFound 表示 Get / Has / Get-Tx 找不到对应 key。
	ErrNotFound = errors.New("xpersistence: key not found")

	// ErrCorrupted 表示文件已损坏,无法继续读取;调用方
	// 应该把数据目录隔离并发出告警,不应试图自愈。
	ErrCorrupted = errors.New("xpersistence: store corrupted")

	// ErrClosed 表示 KV 实例已经 Close 之后再调用。
	ErrClosed = errors.New("xpersistence: kv closed")

	// ErrAtomicFn 表示事务回调返回错误,所有改动回滚。
	ErrAtomicFn = errors.New("xpersistence: atomic fn failed")
)

// Entry 是 List + Get 的组合返回值,用于一次性
// 拉多条记录。文件后端会尽量避免重复 fsync。
type Entry struct {
	Key   string
	Value []byte
}
