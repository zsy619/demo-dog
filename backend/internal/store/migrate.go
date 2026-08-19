package store

// Schema migration framework.
//
// As the store evolves, the on-disk layout of snapshots and
// WAL records changes too. Without a migration framework every
// operator has to run a manual conversion on upgrade, which is
// fragile and unscalable.
//
// The framework here is intentionally minimal:
//
//   * Each migration has a unique integer version and a Name.
//   * Apply takes the current persisted bytes and returns the
//     new bytes + new version.
//   * Migrator.Apply walks the chain from the source version
//     to the head, picking only the migrations in between.
//   * Persistence writes the latest version alongside the data
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

// Migration is one step in the chain.
type Migration struct {
	Version int
	Name    string
	Apply   func(payload []byte) ([]byte, error)
}

// Migrator runs migrations in order.
type Migrator struct {
	chain []Migration
}

// NewMigrator returns a Migrator with the given chain. The
// chain is stored as-is; callers are expected to construct it
// in increasing version order.
func NewMigrator(chain []Migration) *Migrator {
	return &Migrator{chain: chain}
}

// Head returns the latest version in the chain.
func (m *Migrator) Head() int {
	if len(m.chain) == 0 {
		return 0
	}
	return m.chain[len(m.chain)-1].Version
}

// Apply runs every migration between from and Head.
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

// EncodeMigrationHeader prefixes payload with a fixed-size
// big-endian uint32 version. The Doris snapshot writer emits
// this header; the migration loader reads it via
// DecodeMigrationHeader to learn the source version.
func EncodeMigrationHeader(version int, payload []byte) []byte {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(version))
	return append(hdr[:], payload...)
}

// DecodeMigrationHeader reads the version prefix.
func DecodeMigrationHeader(payload []byte) (int, []byte, error) {
	if len(payload) < 4 {
		return 0, nil, errors.New("payload too short for migration header")
	}
	v := binary.BigEndian.Uint32(payload[:4])
	return int(v), payload[4:], nil
}

// ---- built-in migrations ----

// V1ToV2: the v1 snapshot was a single JSON document; v2
// adds a histogram list at the end under PersistHistogram.
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

// V2ToV3: adds an empty TDigestMap. Idempotent.
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

// DefaultChain returns the chain shipped with the binary. New
// migrations should be appended; never reorder.
func DefaultChain() []Migration {
	return []Migration{
		{Version: 1, Name: "v1-baseline", Apply: func(p []byte) ([]byte, error) { return p, nil }},
		{Version: 2, Name: "add-histograms", Apply: V1ToV2},
		{Version: 3, Name: "add-tdigests", Apply: V2ToV3},
	}
}
