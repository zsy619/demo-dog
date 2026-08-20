package store

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// BackupManager 负责备份创建与恢复。
type BackupManager struct {
	mu      sync.Mutex
	dataDir string
}

// BackupOptions 自定义一次备份运行。
type BackupOptions struct {
	Output     string
	Compress   *bool
	IncludeWAL *bool
}

// BackupResult 汇总已写入的内容。
type BackupResult struct {
	Output      string    `json:"output"`
	Bytes       int64     `json:"bytes"`
	SHA256      string    `json:"sha256"`
	SnapshotID  string    `json:"snapshot_id"`
	TakenAt     time.Time `json:"taken_at"`
	Compress    bool      `json:"compress"`
	WALIncluded bool      `json:"wal_included"`
}

// NewBackupManager 为给定数据目录创建管理器。
func NewBackupManager(dataDir string) *BackupManager {
	return &BackupManager{dataDir: dataDir}
}

// Backup 写入一个自包含的归档。
func (m *BackupManager) Backup(store *Doris, opts BackupOptions) (BackupResult, error) {
	if opts.Output == "" {
		return BackupResult{}, errors.New("output path required")
	}
	if opts.Compress == nil {
		t := true
		opts.Compress = &t
	}
	if opts.IncludeWAL == nil {
		t := true
		opts.IncludeWAL = &t
	}
	res := BackupResult{Output: opts.Output, TakenAt: time.Now(), Compress: *opts.Compress, WALIncluded: *opts.IncludeWAL}
	out, err := os.Create(opts.Output)
	if err != nil {
		return res, fmt.Errorf("create %s: %w", opts.Output, err)
	}
	defer out.Close()
	hash := sha256.New()
	w := io.MultiWriter(out, hash)
	header := backupHeader{Version: 1, Magic: backupMagic}
	if err := writeHeader(w, header); err != nil {
		return res, fmt.Errorf("write header: %w", err)
	}
	var files []backupFileEntry
	ts := time.Now().UTC()
	snapID := fmt.Sprintf("snap-%s", ts.Format("20060102T150405"))
	res.SnapshotID = snapID
	snapBytes, err := store.PersistSnapshotBytes()
	if err != nil {
		return res, fmt.Errorf("snapshot: %w", err)
	}
	if err := writeEntry(w, "snapshot.bin", snapBytes, *opts.Compress); err != nil {
		return res, err
	}
	files = append(files, backupFileEntry{Name: "snapshot.bin", Compressed: *opts.Compress, Bytes: int64(len(snapBytes))})
	if *opts.IncludeWAL {
		walBytes, err := readWALFile(m.dataDir)
		if err != nil {
			return res, fmt.Errorf("wal read: %w", err)
		}
		if len(walBytes) > 0 {
			if err := writeEntry(w, "wal.bin", walBytes, *opts.Compress); err != nil {
				return res, err
			}
			files = append(files, backupFileEntry{Name: "wal.bin", Compressed: *opts.Compress, Bytes: int64(len(walBytes))})
		}
	}
	meta := backupMeta{Version: 2, SnapshotID: snapID, TakenAt: ts}
	metaBytes, _ := json.Marshal(meta)
	if err := writeEntry(w, "metadata.json", metaBytes, false); err != nil {
		return res, err
	}
	files = append(files, backupFileEntry{Name: "metadata.json", Bytes: int64(len(metaBytes))})
	manifestBytes, _ := json.Marshal(files)
	if err := writeEntry(w, "manifest.json", manifestBytes, false); err != nil {
		return res, err
	}
	totalHash := hash.Sum(nil)
	res.SHA256 = hex.EncodeToString(totalHash)
	info, err := out.Stat()
	if err == nil {
		res.Bytes = info.Size()
	}
	return res, nil
}

// RestoreOption 自定义 Restore。
type RestoreOption func(*backupOptions)

type backupOptions struct {
	DryRun  bool
	IntoDir string
}

// RestoreResult 汇总已恢复的内容。
type RestoreResult struct {
	Input         string    `json:"input"`
	SnapshotID    string    `json:"snapshot_id"`
	RestoredFiles []string  `json:"restored_files"`
	TakenAt       time.Time `json:"taken_at"`
	SHA256        string    `json:"sha256"`
}

// Restore 读取备份归档。
func (m *BackupManager) Restore(input string, opts ...RestoreOption) (RestoreResult, error) {
	res := RestoreResult{Input: input}
	cfg := backupOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.IntoDir == "" {
		cfg.IntoDir = m.dataDir
	}
	in, err := os.Open(input)
	if err != nil {
		return res, fmt.Errorf("open %s: %w", input, err)
	}
	defer in.Close()
	hash := sha256.New()
	br := io.TeeReader(in, hash)
	var hdr backupHeader
	if err := readHeader(br, &hdr); err != nil {
		return res, fmt.Errorf("read header: %w", err)
	}
	if hdr.Magic != backupMagic {
		return res, errors.New("not a demo-dog backup file")
	}
	if !cfg.DryRun {
		if err := os.MkdirAll(cfg.IntoDir, 0o755); err != nil {
			return res, err
		}
	}
	files := map[string][]byte{}
	for {
		entry, data, err := readEntry(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, fmt.Errorf("read entry: %w", err)
		}
		if !cfg.DryRun {
			path := filepath.Join(cfg.IntoDir, entry.Name)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return res, fmt.Errorf("write %s: %w", entry.Name, err)
			}
		}
		res.RestoredFiles = append(res.RestoredFiles, entry.Name)
		files[entry.Name] = data
	}
	if metaBytes, ok := files["metadata.json"]; ok {
		var meta backupMeta
		if err := json.Unmarshal(metaBytes, &meta); err == nil {
			res.SnapshotID = meta.SnapshotID
			res.TakenAt = meta.TakenAt
		}
	}
	res.SHA256 = hex.EncodeToString(hash.Sum(nil))
	sort.Strings(res.RestoredFiles)
	return res, nil
}

// Verify 检查归档魔术字并遍历条目。
func (m *BackupManager) Verify(input string) error {
	in, err := os.Open(input)
	if err != nil {
		return err
	}
	defer in.Close()
	hash := sha256.New()
	br := io.TeeReader(in, hash)
	var hdr backupHeader
	if err := readHeader(br, &hdr); err != nil {
		return err
	}
	if hdr.Magic != backupMagic {
		return errors.New("not a demo-dog backup file")
	}
	for {
		_, _, err := readEntry(br)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// ListBackups 枚举目录中的备份文件。
func ListBackups(dir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []BackupInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".bak") && !strings.HasSuffix(e.Name(), ".backup") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{Name: e.Name(), Path: filepath.Join(dir, e.Name()), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

// BackupInfo 是备份目录列表中的一项。
type BackupInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// RestoreDryRun 返回不写入的 RestoreOption。
func RestoreDryRun() RestoreOption {
	return func(o *backupOptions) { o.DryRun = true }
}

// RestoreIntoDir 返回提取到自定义目录的 RestoreOption。
func RestoreIntoDir(dir string) RestoreOption {
	return func(o *backupOptions) { o.IntoDir = dir }
}

const backupMagic = "DOGBACKUP\x00"

type backupHeader struct {
	Magic   string `json:"magic"`
	Version int    `json:"version"`
}

type backupFileEntry struct {
	Name       string `json:"name"`
	Compressed bool   `json:"compressed,omitempty"`
	Bytes      int64  `json:"bytes"`
}

type backupMeta struct {
	Version    int       `json:"version"`
	SnapshotID string    `json:"snapshot_id"`
	TakenAt    time.Time `json:"taken_at"`
}

func writeHeader(w io.Writer, h backupHeader) error {
	b, _ := json.Marshal(h)
	if len(b) > 64 {
		return errors.New("header too large")
	}
	padded := make([]byte, 64)
	copy(padded, b)
	for i := len(b); i < 64; i++ {
		padded[i] = ' '
	}
	_, err := w.Write(padded)
	return err
}

func readHeader(r io.Reader, h *backupHeader) error {
	padded := make([]byte, 64)
	if _, err := io.ReadFull(r, padded); err != nil {
		return err
	}
	return json.Unmarshal(padded, h)
}

type entryFrame struct {
	Name       string `json:"name"`
	Compressed bool   `json:"compressed,omitempty"`
	Length     int64  `json:"length"`
}

func writeEntry(w io.Writer, name string, data []byte, compress bool) error {
	var payload []byte
	if compress {
		gz := newGZBuffer()
		if _, err := gz.Write(data); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}
		payload = gz.Bytes()
	} else {
		payload = data
	}
	frame := entryFrame{Name: name, Compressed: compress, Length: int64(len(payload))}
	frameBytes, _ := json.Marshal(frame)
	var lenBuf [8]byte
	var n int64 = int64(len(frameBytes))
	for i := 7; i >= 0; i-- {
		lenBuf[i] = byte(n & 0xff)
		n >>= 8
	}
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(frameBytes); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}

func readEntry(r io.Reader) (entryFrame, []byte, error) {
	var lenBuf [8]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return entryFrame{}, nil, err
	}
	var frameLen int64
	for i := 0; i < 8; i++ {
		frameLen = frameLen<<8 | int64(lenBuf[i])
	}
	frameBytes := make([]byte, frameLen)
	if _, err := io.ReadFull(r, frameBytes); err != nil {
		return entryFrame{}, nil, err
	}
	var frame entryFrame
	if err := json.Unmarshal(frameBytes, &frame); err != nil {
		return frame, nil, err
	}
	buf := make([]byte, frame.Length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return frame, nil, err
	}
	if frame.Compressed {
		gz, err := gzip.NewReader(bytesReader(buf))
		if err != nil {
			return frame, nil, err
		}
		defer gz.Close()
		dec, err := io.ReadAll(gz)
		if err != nil {
			return frame, nil, err
		}
		return frame, dec, nil
	}
	return frame, buf, nil
}

type gzBuffer struct {
	buf []byte
	gz  *gzip.Writer
}

func newGZBuffer() *gzBuffer {
	b := &gzBuffer{}
	b.gz = gzip.NewWriter(writerFunc(func(p []byte) (int, error) {
		b.buf = append(b.buf, p...)
		return len(p), nil
	}))
	return b
}

func (b *gzBuffer) Write(p []byte) (int, error) { return b.gz.Write(p) }
func (b *gzBuffer) Close() error                { return b.gz.Close() }
func (b *gzBuffer) Bytes() []byte               { return b.buf }

type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func bytesReader(b []byte) io.Reader { return &sliceReader{b: b} }

type sliceReader struct {
	b   []byte
	off int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.off >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.off:])
	r.off += n
	return n, nil
}

// readWALFile reads the WAL file in dataDir (if it exists).
func readWALFile(dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, "demo-dog.wal")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
