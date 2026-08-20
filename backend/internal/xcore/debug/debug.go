// Package debug 调试端点：暴露内部状态（pprof 之外的调试信息）。
package debug

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type Gate struct {
	mu        sync.RWMutex
	token     string
	tokenHash string
	open      bool
	hits      int
	reject    int
}

func NewGate(rawToken string) *Gate {
	g := &Gate{}
	g.setToken(rawToken)
	return g
}

func (g *Gate) setToken(raw string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if raw == "" {
		g.open = true
		g.token = ""
		g.tokenHash = ""
		return
	}
	g.open = false
	g.token = raw
	h := sha256.Sum256([]byte(raw))
	g.tokenHash = hex.EncodeToString(h[:])
}

func (g *Gate) Allow(r *http.Request) error {
	g.mu.RLock()
	open := g.open
	got := sha256.Sum256([]byte(r.Header.Get("X-Debug-Token")))
	gotHex := hex.EncodeToString(got[:])
	g.mu.RUnlock()
	g.mu.Lock()
	defer g.mu.Unlock()
	if open {
		g.hits++
		return nil
	}
	if gotHex == g.tokenHash && g.tokenHash != "" {
		g.hits++
		return nil
	}
	g.reject++
	return errors.New("unauthorised")
}

type GateStats struct {
	Open   bool `json:"open"`
	Hits   int  `json:"hits"`
	Reject int  `json:"reject"`
}

func (g *Gate) Stats() GateStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return GateStats{Open: g.open, Hits: g.hits, Reject: g.reject}
}

type Version struct {
	Service   string    `json:"service"`
	BuildSHA  string    `json:"build_sha"`
	BuildTime time.Time `json:"build_time"`
	GoVersion string    `json:"go_version"`
	Now       time.Time `json:"now"`
}

func Handler(gate *Gate, v Version) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/version", versionHandler(v))
	mux.HandleFunc("/debug/info", infoHandler(v))
	mux.HandleFunc("/debug/stack", stackHandler())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := gate.Allow(r); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func versionHandler(v Version) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v.Now = time.Now()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"service":%q,"build_sha":%q,"go_version":%q,"now":%q}`,
			v.Service, v.BuildSHA, v.GoVersion, v.Now.Format(time.RFC3339))
	}
}

func infoHandler(v Version) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		fmt.Fprintf(w, "go_version=%s\n", v.GoVersion)
		fmt.Fprintf(w, "goroutines=%d\n", runtime.NumGoroutine())
		fmt.Fprintf(w, "heap_alloc=%d\n", ms.HeapAlloc)
		fmt.Fprintf(w, "heap_sys=%d\n", ms.HeapSys)
		fmt.Fprintf(w, "heap_objects=%d\n", ms.HeapObjects)
		fmt.Fprintf(w, "num_gc=%d\n", ms.NumGC)
		fmt.Fprintf(w, "gc_pause_total_ns=%d\n", ms.PauseTotalNs)
	}
}

func stackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		w.Header().Set("Content-Type", "text/plain")
		w.Write(buf[:n])
	}
}
