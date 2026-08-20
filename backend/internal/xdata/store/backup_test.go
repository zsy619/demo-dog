package store

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestDoris(t *testing.T) (*Doris, string) {
	t.Helper()
	return New(DefaultConfig()), t.TempDir()
}

func TestBackup_BackupRestore_RoundTrip(t *testing.T) {
	d, dir := newTestDoris(t)
	bm := NewBackupManager(dir)
	out := filepath.Join(t.TempDir(), "test.bak")
	res, err := bm.Backup(d, BackupOptions{Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.SHA256 == "" {
		t.Fatal("expected SHA256")
	}
	if res.SnapshotID == "" {
		t.Fatal("expected snapshot id")
	}
	if res.Bytes == 0 {
		t.Fatal("expected non-zero bytes")
	}

	dest := t.TempDir()
	rres, err := bm.Restore(out, RestoreIntoDir(dest))
	if err != nil {
		t.Fatal(err)
	}
	if len(rres.RestoredFiles) == 0 {
		t.Fatal("expected restored files")
	}
	if rres.SnapshotID != res.SnapshotID {
		t.Fatalf("snapshot id: %q vs %q", rres.SnapshotID, res.SnapshotID)
	}
}

func TestBackup_NoCompress(t *testing.T) {
	d, dir := newTestDoris(t)
	bm := NewBackupManager(dir)
	out := filepath.Join(t.TempDir(), "test.bak")
	f := false
	res, err := bm.Backup(d, BackupOptions{Output: out, Compress: &f})
	if err != nil {
		t.Fatal(err)
	}
	if res.Compress {
		t.Fatal("Compress=false should result in res.Compress=false")
	}
}

func TestBackup_Verify(t *testing.T) {
	d, dir := newTestDoris(t)
	bm := NewBackupManager(dir)
	out := filepath.Join(t.TempDir(), "test.bak")
	if _, err := bm.Backup(d, BackupOptions{Output: out}); err != nil {
		t.Fatal(err)
	}
	if err := bm.Verify(out); err != nil {
		t.Fatal(err)
	}
}

func TestBackup_Verify_BadMagic(t *testing.T) {
	out := filepath.Join(t.TempDir(), "junk.bak")
	if err := os.WriteFile(out, []byte("definitely not a backup file"), 0o644); err != nil {
		t.Fatal(err)
	}
	bm := NewBackupManager("")
	if err := bm.Verify(out); err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestBackup_DryRun(t *testing.T) {
	d, dir := newTestDoris(t)
	bm := NewBackupManager(dir)
	out := filepath.Join(t.TempDir(), "test.bak")
	if _, err := bm.Backup(d, BackupOptions{Output: out}); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	rres, err := bm.Restore(out, RestoreIntoDir(dest), RestoreDryRun())
	if err != nil {
		t.Fatal(err)
	}
	if len(rres.RestoredFiles) == 0 {
		t.Fatal("dry run should report file list")
	}
	entries, _ := os.ReadDir(dest)
	if len(entries) != 0 {
		t.Fatal("dry run should not write files")
	}
}

func TestBackup_IncludeWAL_NoFile(t *testing.T) {
	d, dir := newTestDoris(t)
	bm := NewBackupManager(dir)
	out := filepath.Join(t.TempDir(), "test.bak")
	res, err := bm.Backup(d, BackupOptions{Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if !res.WALIncluded {
		t.Fatal("expected WALIncluded=true")
	}
}

func TestBackup_IncludeWAL_WithFile(t *testing.T) {
	d, dir := newTestDoris(t)
	walPath := filepath.Join(dir, "demo-dog.wal")
	if err := os.WriteFile(walPath, []byte("1234567890"), 0o644); err != nil {
		t.Fatal(err)
	}
	bm := NewBackupManager(dir)
	out := filepath.Join(t.TempDir(), "test.bak")
	if _, err := bm.Backup(d, BackupOptions{Output: out}); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if _, err := bm.Restore(out, RestoreIntoDir(dest)); err != nil {
		t.Fatal(err)
	}
	walBytes, err := os.ReadFile(filepath.Join(dest, "wal.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(walBytes, []byte("1234567890")) {
		t.Fatal("expected wal bytes in restore")
	}
}

func TestBackup_NoOutputPath(t *testing.T) {
	d, dir := newTestDoris(t)
	bm := NewBackupManager(dir)
	if _, err := bm.Backup(d, BackupOptions{}); err == nil {
		t.Fatal("expected error for missing output")
	}
}

func TestBackup_OutputRequired(t *testing.T) {
	d, dir := newTestDoris(t)
	bm := NewBackupManager(dir)
	if _, err := bm.Backup(d, BackupOptions{Output: "/nonexistent/dir/should/fail.bak"}); err == nil {
		t.Fatal("expected error for bad output path")
	}
}

func TestListBackups_FilterSuffix(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.bak"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.backup"), nil, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "c.txt"), nil, 0o644)
	list, err := ListBackups(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 (filter .txt), got %d", len(list))
	}
}

func TestListBackups_Empty(t *testing.T) {
	list, err := ListBackups(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty, got %d", len(list))
	}
}

func TestListBackups_MissingDir(t *testing.T) {
	if _, err := ListBackups("/nonexistent/path/should/fail"); err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestBackup_BigContent(t *testing.T) {
	d, dir := newTestDoris(t)
	walPath := filepath.Join(dir, "demo-dog.wal")
	big := make([]byte, 100*1024)
	if _, err := rand.Read(big); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walPath, big, 0o644); err != nil {
		t.Fatal(err)
	}
	bm := NewBackupManager(dir)
	out := filepath.Join(t.TempDir(), "test.bak")
	res, err := bm.Backup(d, BackupOptions{Output: out})
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes < int64(len(big)) {
		t.Fatal("compressed file should be >= raw size")
	}
}

func fileStartsWithMagic(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, len(backupMagic))
	if _, err := f.Read(head); err != nil {
		return false
	}
	return strings.HasPrefix(string(head), backupMagic[:len(head)])
}
