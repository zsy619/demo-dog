package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		name string
		c    Config
		err  bool
	}{
		{"valid", Config{IngestAddr: ":1", Workers: 1, Sampling: 0.5}, false},
		{"no addr", Config{Workers: 1, Sampling: 0.5}, true},
		{"no workers", Config{IngestAddr: ":1", Sampling: 0.5}, true},
		{"bad sampling", Config{IngestAddr: ":1", Workers: 1, Sampling: 1.5}, true},
		{"bad sampling low", Config{IngestAddr: ":1", Workers: 1, Sampling: -0.1}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.c.Validate()
			if (err != nil) != c.err {
				t.Fatalf("err=%v want %v", err, c.err)
			}
		})
	}
}

func TestDefault(t *testing.T) {
	d := Default()
	if d.IngestAddr == "" || d.Workers == 0 {
		t.Fatal("defaults")
	}
}

func TestParse_Empty(t *testing.T) {
	c, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.IngestAddr == "" {
		t.Fatal("defaults should apply")
	}
}

func TestParse_OverridesDefaults(t *testing.T) {
	c, err := Parse([]byte(`{"ingest_addr":":9090","workers":4,"sampling_rate":0.25}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.IngestAddr != ":9090" || c.Workers != 4 || c.Sampling != 0.25 {
		t.Fatalf("got %+v", c)
	}
}

func TestParse_BadJSON(t *testing.T) {
	if _, err := Parse([]byte("not json")); err == nil {
		t.Fatal("expected error")
	}
}

func TestParse_BadValidate(t *testing.T) {
	if _, err := Parse([]byte(`{"sampling_rate":2}`)); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	if err := os.WriteFile(p, []byte(`{"ingest_addr":":8080","workers":2,"sampling_rate":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Workers != 2 {
		t.Fatal("loaded")
	}
}

func TestLoad_Missing(t *testing.T) {
	if _, err := Load("/no/such/file"); err == nil {
		t.Fatal("expected error")
	}
}

func TestHash_Stable(t *testing.T) {
	c := Config{IngestAddr: ":1", Workers: 1, Sampling: 0.5}
	if c.Hash() != c.Hash() {
		t.Fatal("hash should be stable")
	}
	c2 := c
	c2.Workers = 2
	if c.Hash() == c2.Hash() {
		t.Fatal("different configs should differ")
	}
}

func TestWatcher_RunOnce(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	if err := os.WriteFile(p, []byte(`{"ingest_addr":":8080","workers":4,"sampling_rate":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var got []Config
	w := NewWatcher(p, 50*time.Millisecond, func(c Config) {
		mu.Lock()
		got = append(got, c)
		mu.Unlock()
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	// 等待首次应用。
	time.Sleep(80 * time.Millisecond)
	// 更新文件。
	if err := os.WriteFile(p, []byte(`{"ingest_addr":":9090","workers":2,"sampling_rate":0.5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if len(got) < 2 {
		t.Fatalf("expected >=2 applies, got %d", len(got))
	}
	if got[len(got)-1].IngestAddr != ":9090" {
		t.Fatal("latest config should be :9090")
	}
}

func TestWatcher_StopsOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	os.WriteFile(p, []byte(`{"ingest_addr":":8080","workers":1,"sampling_rate":1}`), 0o644)
	w := NewWatcher(p, 50*time.Millisecond, func(Config) {})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop")
	}
}

func TestWatcher_BadFile_RecordsError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "missing.json")
	w := NewWatcher(p, 50*time.Millisecond, func(Config) {})
	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from missing file")
	}
	if w.Stats().Errors == 0 {
		t.Fatal("error counter")
	}
}

func TestWatcher_DoubleRunFails(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	os.WriteFile(p, []byte(`{"ingest_addr":":8080","workers":1,"sampling_rate":1}`), 0o644)
	w := NewWatcher(p, 50*time.Millisecond, func(Config) {})
	go w.Run(context.Background())
	time.Sleep(20 * time.Millisecond)
	var got error
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		got = w.Run(context.Background())
		mu.Lock()
		close(done)
		mu.Unlock()
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return")
	}
	w.Stop()
	mu.Lock()
	defer mu.Unlock()
	if got == nil {
		t.Fatal("expected double-run error")
	}
}

func TestWatcher_Stop(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	os.WriteFile(p, []byte(`{"ingest_addr":":8080","workers":1,"sampling_rate":1}`), 0o644)
	w := NewWatcher(p, 50*time.Millisecond, func(Config) {})
	done := make(chan error, 1)
	go func() { done <- w.Run(context.Background()) }()
	time.Sleep(20 * time.Millisecond)
	w.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("did not stop")
	}
}

func TestWatcher_Current(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	os.WriteFile(p, []byte(`{"ingest_addr":":8080","workers":2,"sampling_rate":1}`), 0o644)
	w := NewWatcher(p, 50*time.Millisecond, func(Config) {})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	time.Sleep(80 * time.Millisecond)
	c := w.Current()
	if c.Workers != 2 {
		t.Fatal("current")
	}
}
