package store

import (
	"crypto/rand"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/zsy619/demo-dog/backend/internal/xdata/model"
)

// TestWAL_RoundTrip writes 100 entries and verifies replay recovers all.
func TestWAL_ChaosRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.bin")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if err := w.Append(opLog, []model.LogRecord{{Service: "svc", Body: "hi"}}); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.Close()

	w, err = OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	logs, _, _, err := w.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 100 {
		t.Fatalf("expected 100 logs, got %d", len(logs))
	}
}

// TestWAL_TruncatedTail simulates a crash mid-write: the file has a
// valid frame followed by a partial header. The next OpenWAL must
// truncate the partial frame so replay recovers all complete frames.
func TestWAL_ChaosTruncatedTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.bin")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	// Write 5 complete frames.
	for i := 0; i < 5; i++ {
		if err := w.Append(opLog, []model.LogRecord{{Body: "x"}}); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.Close()

	// Append 12 bytes of garbage: looks like the start of a frame but
	// not enough to satisfy the body.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	buf := []byte{0x00, 0x00, 0x0D, 0x06, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01}
	if _, err := f.Write(buf); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// Reopen; repairWAL should truncate the partial tail.
	w, err = OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	logs, _, _, err := w.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 5 {
		t.Fatalf("expected 5 logs after truncation, got %d", len(logs))
	}
}

// TestWAL_BadMagic drops a magic prefix and confirms we stop reading
// without panicking. Anything after is irrelevant.
func TestWAL_BadMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte{0xDE, 0xAD, 0xBE, 0xEF})
	_ = f.Close()

	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	logs, _, _, err := w.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs from a corrupt WAL, got %d", len(logs))
	}
}

// TestWAL_OversizeLength declares a length field of 1 GiB; the repair
// step must treat it as corruption and stop.
func TestWAL_OversizeLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	hdr := make([]byte, 16)
	binary.BigEndian.PutUint32(hdr[0:4], walMagic)
	binary.BigEndian.PutUint32(hdr[4:8], walVersion)
	binary.BigEndian.PutUint32(hdr[8:12], opLog)
	binary.BigEndian.PutUint32(hdr[12:16], 1<<30) // 1 GiB
	_, _ = f.Write(hdr)
	_ = f.Close()

	if _, err := OpenWAL(path); err != nil {
		t.Fatal(err)
	}
	// We just want to confirm OpenWAL doesn't OOM. The repair step
	// truncates the file (it may be 0 bytes if good is 0, or 16
	// bytes if the magic was valid but the length field was insane).
	stat, _ := os.Stat(path)
	if stat.Size() > 16 {
		t.Fatalf("expected file truncated, got size=%d", stat.Size())
	}
}

// TestWAL_FuzzSurvivors writes 50 frames, then plants a single bad
// byte somewhere in the middle. Replay should recover at least the
// frames before the corruption and silently stop at the bad point.
func TestWAL_FuzzSurvivors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.bin")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		_ = w.Append(opMetric, []model.MetricPoint{{Name: "n", Value: float64(i)}})
	}
	_ = w.Close()

	// Plant 32 random bytes in the middle.
	f, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	stat, _ := f.Stat()
	if stat.Size() < 100 {
		t.Fatal("file too small to plant")
	}
	if _, err := f.Seek(stat.Size()/2, 0); err != nil {
		t.Fatal(err)
	}
	bad := make([]byte, 32)
	_, _ = rand.Read(bad)
	_, _ = f.Write(bad)
	_ = f.Close()

	// OpenWAL truncates at the bad magic. Replay sees only the prefix.
	if _, err := OpenWAL(path); err != nil {
		t.Fatal(err)
	}
	w, _ = OpenWAL(path)
	_, metrics, _, err := w.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected some prefix frames to survive")
	}
	if len(metrics) >= 50 {
		t.Fatalf("expected <50 metrics (corruption should drop tail), got %d", len(metrics))
	}
}

// TestWAL_EmptyFile opens an empty WAL; must not crash.
func TestWAL_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	logs, metrics, spans, err := w.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs)+len(metrics)+len(spans) != 0 {
		t.Fatal("empty WAL should replay empty")
	}
}

// TestWAL_RotateThenAppend verifies the snapshot-rotate-write path:
// after Rotate() the file is empty but a fresh Append still works.
func TestWAL_ChaosRotateThenAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.bin")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Append(opLog, []model.LogRecord{{Body: "old"}})
	if err := w.Rotate(); err != nil {
		t.Fatal(err)
	}
	_ = w.Append(opLog, []model.LogRecord{{Body: "new"}})
	_ = w.Close()

	w, err = OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	logs, _, _, err := w.Replay()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Body != "new" {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
}
