package store

// Schema 迁移框架。
//
// As the store evolves, the on-disk layout of snapshots and
// WAL 记录 changes too. Without a migration framework every
// operator has to run a manual conversion on upgrade, which is
// fragile and unscalable.
//
// The framework here is intentionally minimal:
//
//   * Each migration has a unique integer version and a Name.
//   * Apply takes the current persisted bytes and 返回
//     new bytes + new version.
//   * Migrator.Apply walks the chain from the source version
//     to the head, picking only the migrations in between.
//   * Persistence 写入 最近的 version alongside the data
//     so Apply can resume on crash.
//
// The migration functions live in this package and are pure:
// no side effects beyond reading + writing the byte payload.
// Wiring into the Doris store is in cmd/dog-collector.

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// Migration 是链中的一个步骤。
type Migration struct {
	Version int
	Name    string
	Apply   func(payload []byte) ([]byte, error)
}

// Migrator 按顺序运行迁移。
type Migrator struct {
	chain []Migration
}

// NewMigrator 返回带给定链的 Migrator。该
// chain is stored as-is; callers are expected to construct it
// in increasing version order.
func NewMigrator(chain []Migration) *Migrator {
	return &Migrator{chain: chain}
}

// Head 返回链中的最新版本。
func (m *Migrator) Head() int {
	if len(m.chain) == 0 {
		return 0
	}
	return m.chain[len(m.chain)-1].Version
}

// Apply 运行从 from 到 Head 之间的每个迁移。
func (m *Migrator) Apply(payload []byte, from int) ([]byte, int, error) {
	if from > m.Head() {
		return nil, 0, fmt.Errorf("source version %d > head %d", from, m.Head())
	}
	cur := payload
	for _, mig := range m.chain {
		if mig.Version <= from {
			continue
		}
		next, err := mig.Apply(cur)
		if err != nil {
			return nil, 0, fmt.Errorf("migration %s (v%d): %w", mig.Name, mig.Version, err)
		}
		cur = next
	}
	return cur, m.Head(), nil
}

// EncodeMigrationHeader 在负载前添加固定大小的
// big-endian uint32 version. The Doris 快照 writer 发出
// this header; the migration loader 读取 it via
// DecodeMigrationHeader to learn the source version.
func EncodeMigrationHeader(version int, payload []byte) []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(version))
	return append(hdr[:], payload...)
}

// DecodeMigrationHeader 读取版本前缀。
func DecodeMigrationHeader(payload []byte) (int, []byte, error) {
	if len(payload) < 4 {
		return 0, nil, errors.New("payload too short for migration header")
	}
	v := binary.BigEndian.Uint32(payload[:4])
	return int(v), payload[4:], nil
}

// ---- built-in migrations ----

// V1ToV2：v1 快照是单个 JSON 文档；v2
// adds a 直方图 list at the end under PersistHistogram.
// We accept v1 input as the older shape and re-marshal it with
// empty histograms.
func V1ToV2(payload []byte) ([]byte, error) {
	var v1 map[string]json.RawMessage
	if err := json.Unmarshal(payload, &v1); err != nil {
		return nil, fmt.Errorf("v1 unmarshal: %w", err)
	}
	if _, ok := v1["Histograms"]; !ok {
		v1["Histograms"] = json.RawMessage("[]")
	}
	return json.Marshal(v1)
}

// V2ToV3：添加一个空 TDigestMap。幂等。
func V2ToV3(payload []byte) ([]byte, error) {
	var v2 map[string]json.RawMessage
	if err := json.Unmarshal(payload, &v2); err != nil {
		return nil, fmt.Errorf("v2 unmarshal: %w", err)
	}
	if _, ok := v2["TDigests"]; !ok {
		v2["TDigests"] = json.RawMessage("[]")
	}
	return json.Marshal(v2)
}

// DefaultChain 返回随二进制发布的迁移链。新的
// migrations should be appended; never reorder.
func DefaultChain() []Migration {
	return []Migration{
		{Version: 1, Name: "v1-baseline", Apply: func(p []byte) ([]byte, error) { return p, nil }},
		{Version: 2, Name: "add-histograms", Apply: V1ToV2},
		{Version: 3, Name: "add-tdigests", Apply: V2ToV3},
	}
}
