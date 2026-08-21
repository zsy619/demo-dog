// Command dog-collector is the DOG Collectors command-line entry point.
//
// It wires the in-memory Doris engine, the OTLP ingest pipeline, the worker
// pool, the WebSocket pub/sub, and the HTTP API together. The number of
// workers, queue depth, and HTTP listen address can be tuned via flags.
//
// Usage:
//
//	go run ./cmd/dog-collector
//	go run ./cmd/dog-collector -workers 16 -addr :9090
//
// The shell prints a banner that previews every endpoint and a few example
// curl commands so the user can interact with the API immediately.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/xnet/api"
	"github.com/zsy619/demo-dog/backend/internal/xdata/ingest"
	"github.com/zsy619/demo-dog/backend/internal/xdata/store"
	"github.com/zsy619/demo-dog/backend/internal/xdata/tenants"
	"github.com/zsy619/demo-dog/backend/internal/xflow/stream"
	"github.com/zsy619/demo-dog/backend/internal/xpersistence"
	"github.com/zsy619/demo-dog/backend/internal/xsecure/auth"
)

// banner is printed once at startup. It foreshadoows every endpoint and gives
// the user a one-liner to seed test data.
const banner = `
 ____   ___   ___  
|  _ \ / _ \ / _ \ 
| | | | | | | | | |
| |_| | |_| | |_| |
|____/ \___/ \___/   Doris + OpenTelemetry + Grafana

DOG-collector v0.1.0
====================
  HTTP address      : %s
  Workers           : %d
  Queue depth       : %d
  Hot log TTL       : %s
  Hot log capacity  : %d entries

Endpoints:
`

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	workers := flag.Int("workers", 8, "ingest worker pool size")
	queue := flag.Int("queue", 4096, "ingest queue depth (bounded; backpressure on overflow)")
	seed := flag.String("seed", "", "Optional comma-separated service names to seed on startup")
	apiKeys := flag.String("api-keys", "", "Comma-separated list of accepted API keys. Empty (default) disables auth. Env DOG_API_KEYS is read as a fallback.")
	corsOrigins := flag.String("cors-origins", "", "Comma-separated CORS origin allowlist. Empty (default) allows any origin (dev mode). Use http://localhost:3000 in production.")
	tlsCert := flag.String("tls-cert", "", "Path to TLS certificate (PEM). When set with -tls-key, the server listens with HTTPS.")
	tlsKey := flag.String("tls-key", "", "Path to TLS private key (PEM).")
	ratePerSec := flag.Float64("rate-limit", 0, "Per-IP token bucket refill rate (req/s). 0 disables rate limiting.")
	rateBurst := flag.Float64("rate-burst", 200, "Per-IP token bucket burst capacity.")
	snapshotPath := flag.String("snapshot", "", "Optional path to a gob-encoded store snapshot. On startup the engine restores from it (if present); on shutdown the current state is atomically saved back to the same path.")
	walPath := flag.String("wal", "", "Optional path to the write-ahead log. When set, every insert is appended to the WAL so a crash + restart replays the last few seconds of writes.")
	persistInterval := flag.Duration("persist-interval", 5*time.Minute, "How often to snapshot+rotate the WAL. 0 disables background persistence.")
	pprofToken := flag.String("pprof-token", "", "When set, exposes net/http/pprof endpoints at /debug/pprof/* gated by `?token=<value>`. Empty disables pprof.")
	selfTrace := flag.Bool("self-trace", false, "Record the collectors own requests as OTLP spans and POST them back to /api/ingest/otlp. Useful for self-observability in production.")
	alertsPath := flag.String("alerts-rules", "", "Optional path to a YAML/JSON file of SLO burn-rate rules. Empty disables alerting.")
	tenantsFlag := flag.String("tenants", "", "Optional comma-separated list of <id>:<name> to seed on startup. Tenants registered here get a default admin key derived from -api-keys.")
	// W1: data-dir 指向 tenant + admin key 等配置类状态的 KV
	// 文件所在目录。空值则退化为纯内存模式(R3 之前的
	// 行为,适合纯开发场景)。
	dataDir := flag.String("data-dir", "", "Directory for persistent KV state (tenants, admin keys, etc). Empty = in-memory only. Recommended: ./data")
	flag.Parse()

	cfg := store.DefaultConfig()
	cardinalityCap := flag.Int("cardinality-cap", cfg.MaxCardinality, "Max unique series (label sets) the engine will accept. New label sets past this cap are dropped.")
	cfg.MaxCardinality = *cardinalityCap
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid store config: %v\n", err)
		os.Exit(2)
	}
	s := store.New(cfg)

	// Restore previous state if a snapshot file exists.
	if *snapshotPath != "" {
		if err := s.LoadFromFile(*snapshotPath); err != nil {
			log.Printf("[DOG] snapshot load failed: %v (starting empty)", err)
		} else {
			fmt.Printf("  Snapshot           : %s (loaded)\n", *snapshotPath)
		}
	}
	// Wire WAL. If the snapshot was loaded, replay the WAL on top so
	// any in-flight writes since the last snapshot are recovered.
	var wal *store.WAL
	if *walPath != "" {
		w, err := store.OpenWAL(*walPath)
		if err != nil {
			log.Printf("[DOG] WAL open failed: %v (continuing without persistence)", err)
		} else {
			wal = w
			s.SetWAL(wal)
				if err := s.ReplayInto(wal); err != nil {
				log.Printf("[DOG] WAL replay failed: %v", err)
			} else {
				fmt.Printf("  WAL               : %s (active)\n", *walPath)
			}
		}
	}
	// Background snapshot+WAL rotation.
	if *persistInterval > 0 && *snapshotPath != "" {
		go store.PeriodicPersist(*persistInterval, s, *snapshotPath, wal)
	}

	hub := stream.NewHub()
	in := ingest.New(s, *workers)
	defer in.Close()
	defer func() {
		if wal != nil {
			_ = wal.Close()
		}
	}()

	apiServer := api.New(s, in, hub)

	if origins := splitCSV(*corsOrigins); len(origins) > 0 {
		apiServer.SetAllowedOrigins(origins)
		fmt.Printf("  CORS origins       : %s\n", strings.Join(origins, ", "))
	} else {
		fmt.Println("  CORS origins       : * (dev mode; configure -cors-origins in production)")
	}
	if *ratePerSec > 0 {
		apiServer.SetRateLimit(*ratePerSec, *rateBurst)
		fmt.Printf("  Rate limit         : %.0f req/s burst=%.0f per IP\n", *ratePerSec, *rateBurst)
	}

	// Configure API-key authentication. Sources, in priority order:
	//   1. -api-keys flag (comma-separated)
	//   2. DOG_API_KEYS env var (comma-separated)
	// If both are empty the server starts in dev mode (no auth).
	keys := *apiKeys
	if keys == "" {
		keys = os.Getenv("DOG_API_KEYS")
	}
	if keys != "" {
		apiServer.SetAuthMode(api.AuthModeAPIKey)
		for _, spec := range splitCSV(keys) {
			// Spec format: "<key>" (defaults to writer) or
			// "<key>:<role>" or "<key>:<role>:<label>" or
			// "<key>:<role>:<label>:<tenant>".
			apiServer.Auth().AddFromSpec(spec)
		}
		fmt.Printf("  Auth mode         : api-key (%d key(s) loaded)\n", apiServer.Auth().Count())
	} else {
		fmt.Println("  Auth mode         : off (dev mode; do not expose to public networks)")
	}
	// Optional tenant registry. Each tenant starts empty; the
	// admin can mint keys via /api/tenants/<id>/keys.
	//
	// W1: 当 -data-dir 设置时,registry + admin store 都挂上
	// xpersistence.KV,所有 CRUD 都会落盘,进程重启后保留。
	//
	// 共享同一个 KV 文件;后续 W1.3~W1.4 也会用同一份
	// KV 加新 namespace,不引入新的 IO 通道。
	//
	// 独立于 -api-keys:即使没有 auth,tenant + KV 也能工作,
	// 这样 -tenants 'foo,bar' + -data-dir ./data 就是一个
	// 合法组合。
	//
	// 如果同时设置 -data-dir + -tenants:创建/重建 tenant,
	// 并把新数据写入 KV。
	// 如果只设置 -data-dir (没有 -tenants):仅打开 KV 并
	// 加载上次持久化的 tenant,不创建任何新租户。
	var reg *tenants.Registry
	var adminStore *auth.AdminStore
	var oidcReg *api.OIDCRegistry
	var kv xpersistence.KV
	if *dataDir != "" {
		if err := os.MkdirAll(*dataDir, 0o755); err != nil {
			log.Printf("[DOG] data-dir mkdir failed: %v", err)
			os.Exit(2)
		}
		kvPath := filepath.Join(*dataDir, "control.json")
		var err error
		kv, err = xpersistence.OpenFileJSON(kvPath)
		if err != nil {
			log.Printf("[DOG] KV open failed: %v", err)
			os.Exit(2)
		}
		defer kv.Close()
		if r2, err := tenants.NewWithKV(context.Background(), kv); err == nil {
			reg = r2
		} else {
			log.Printf("[DOG] tenants load failed: %v", err)
			os.Exit(2)
		}
		if s2, err := auth.NewAdminStoreWithKV(context.Background(), kv); err == nil {
			adminStore = s2
		} else {
			log.Printf("[DOG] admin store load failed: %v", err)
			os.Exit(2)
		}
		if o2, err := api.NewOIDCRegistryWithKV(context.Background(), kv); err == nil {
			oidcReg = o2
		} else {
			log.Printf("[DOG] OIDC registry load failed: %v", err)
			os.Exit(2)
		}
		fmt.Printf("  Persistence        : %s (control KV active)\n", kvPath)
	} else if *tenantsFlag != "" {
		// 没有 -data-dir 但有 -tenants:回退纯内存
		reg = tenants.New()
		adminStore = auth.NewAdminStore()
	}
	if *tenantsFlag != "" {
		for _, t := range splitCSV(*tenantsFlag) {
			parts := strings.SplitN(t, ":", 2)
			name := parts[0]
			if len(parts) == 2 {
				name = parts[1]
			}
			if _, err := reg.CreateTenant(parts[0], name, ""); err == nil {
				fmt.Printf("  Tenant            : %s (%s)\n", parts[0], name)
			}
		}
	}
	if reg != nil {
		apiServer.SetTenants(reg)
	}
	if adminStore != nil {
		apiServer.SetAdminKeys(adminStore)
	}
	if oidcReg != nil {
		apiServer.SetOIDC(oidcReg)
	}

	// If seed services are provided, drop a few records before serving so the
	// first dashboard isnt empty.
if *seed != "" {
		for _, name := range splitCSV(*seed) {
			apiServer.InjectSeed(name, 10)
		}
	}

	// Alerts: load rules from a YAML / JSON file when configured,
	// then run a 30s ticker that evaluates burn rates and POSTs
	// webhooks for any fired rules.
	if *alertsPath != "" {
		if rules, err := api.LoadAlertRulesFile(*alertsPath); err == nil {
			apiServer.SetAlertRules(rules)
			fmt.Printf("  Alert rules      : %d loaded from %s\n", len(rules), *alertsPath)
		} else {
			fmt.Printf("  Alert rules      : load failed: %v\n", err)
		}
	}
	go apiServer.RunAlertTicker(30 * time.Second)

	// MountPProf + WrapWithSelfTrace must be called BEFORE we read
	// apiServer.Handler() so the server struct sees the configured
	// flags at chain-build time.
	if *pprofToken != "" {
		apiServer.MountPProf("/debug/pprof", *pprofToken)
	}
	if *selfTrace {
		apiServer.WrapWithSelfTrace("http://localhost" + *addr)
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	fmt.Printf(banner, *addr, *workers, *queue, cfg.HotLogTTL, cfg.HotLogCap)
	printEndpoints(*addr)

	idleClosed := make(chan struct{})
	go func() {
		sigInt := make(chan os.Signal, 1)
		signal.Notify(sigInt, syscall.SIGINT, syscall.SIGTERM)
		<-sigInt
		fmt.Println("\n[DOG] shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// 1. Stop accepting new HTTP requests and wait for in-flight ones.
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("[DOG] graceful http shutdown failed: %v", err)
		}
		// 2. Drain the ingest pipeline so queued signals land in the
		//    store instead of being silently dropped.
		in.Close()
		// 3. Snapshot the in-memory state to disk so the next process
		//    can pick up where we left off.
		if *snapshotPath != "" {
			if err := s.SaveToFile(*snapshotPath); err != nil {
				log.Printf("[DOG] snapshot save failed: %v", err)
			} else {
				fmt.Printf("  Snapshot saved     : %s\n", *snapshotPath)
			}
		}
		close(idleClosed)
	}()

	var listenErr error
	if *tlsCert != "" && *tlsKey != "" {
		fmt.Printf("  TLS                 : %s / %s\n", *tlsCert, *tlsKey)
		listenErr = server.ListenAndServeTLS(*tlsCert, *tlsKey)
	} else {
		listenErr = server.ListenAndServe()
	}
	if listenErr != nil && listenErr != http.ErrServerClosed {
		// We intentionally do NOT use log.Fatalf here: it calls os.Exit,
		// which skips defers and would abandon any in-flight ingest
		// queue / partial batch. Instead, log the error, drain the
		// ingest pipeline, and exit cleanly.
		log.Printf("[DOG] http server error: %v", listenErr)
		in.Close()
		if *snapshotPath != "" {
			if err := s.SaveToFile(*snapshotPath); err != nil {
				log.Printf("[DOG] snapshot save failed: %v", err)
			}
		}
		fmt.Println("[DOG] bye.")
		os.Exit(1)
	}
	<-idleClosed
	fmt.Println("[DOG] bye.")
}

// printEndpoints prints a few handy curl commands for the user.
func printEndpoints(addr string) {
	fmt.Println("  GET  /api/health")
	fmt.Println("  GET  /api/services")
	fmt.Println("  GET  /api/services/{name}")
	fmt.Println("  GET  /api/query?type=logs&service=checkout")
	fmt.Println("  GET  /api/query?type=metrics&service=checkout&name=http.server.duration&window=1m")
	fmt.Println("  GET  /api/query?type=traces&service=checkout")
	fmt.Println("  GET  /api/datasources")
	fmt.Println("  GET  /api/dashboards")
	fmt.Println("  GET  /api/dashboards/overview/panels")
	fmt.Println("  POST /api/ingest/otlp   (OTLP JSON)")
	fmt.Println("  GET  /api/stream         (WebSocket)")
	fmt.Println("  GET  /api/seed?service=checkout&n=20")
	fmt.Println("  GET  /api/seed/stream    (SSE; 1 record per second)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  curl -s %s/api/health | jq\n", addr)
	fmt.Printf("  curl -s %s/api/services | jq\n", addr)
	fmt.Printf("  curl -s %s/api/seed?service=checkout&n=20 | jq\n", addr)
	fmt.Println()
}

// splitCSV splits a comma-separated string into trimmed non-empty parts.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
