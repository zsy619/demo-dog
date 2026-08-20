package wal

// snapshot.go:Snapshot 类型与编码辅助。

import "encoding/json"

// Snapshot 是最近一次快照的状态。
//
// 通过 WriteSnapshot 持久化;通过 LastSnapshot 读取。
type Snapshot struct {
	Seq     uint64 `json:"seq"`     // 快照对应的最后已确认 seq
	Payload []byte `json:"payload"` // 快照负载(引擎状态序列化)
}

// encodeSnapshotBlob 将 Snapshot 序列化为 JSON 字节。
//
// 用于把快照作为一条特殊帧写入 WAL 起始位置(供 Reader 识别)。
func encodeSnapshotBlob(s *Snapshot) []byte {
	b, _ := json.Marshal(s)
	return b
}
