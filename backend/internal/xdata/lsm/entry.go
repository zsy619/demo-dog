// Package lsm LSM 结构:内存 + 多层磁盘的写入优化存储。
//
// 文件职责拆分:
//   - entry.go        Entry 类型
//   - memtable.go     Memtable 内存有序存储
//   - run.go          SortedRun 不可变有序层
//   - string_table.go StringTable memtable + 多层 run
//   - internal.go     私有辅助
package lsm

// Entry 是一个键值对。已删除条目表示为 Tombstone=true。
type Entry struct {
	Key       string // 键
	Value     []byte // 值
	Tombstone bool   // 是否墓碑(逻辑删除)
}
