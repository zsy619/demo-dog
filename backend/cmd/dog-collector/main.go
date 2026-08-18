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
	"syscall"
	"time"

	"github.com/zsy619/demo-dog/backend/internal/api"
	"github.com/zsy619/demo-dog/backend/internal/ingest"
	"github.com/zsy619/demo-dog/backend/internal/store"
	"github.com/zsy619/demo-dog/backend/internal/stream"
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
	flag.Parse()

	cfg := store.DefaultConfig()
	s := store.New(cfg)

	hub := stream.NewHub()
	in := ingest.New(s, *workers)
	defer in.Close()

	apiServer := api.New(s, in, hub)

	// If seed services are provided, drop a few records before serving so the
	// first dashboard isnt empty.
	if *seed != "" {
		for _, name := range splitCSV(*seed) {
			apiServer.InjectSeed(name, 10)
		}
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
		close(idleClosed)
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		// We intentionally do NOT use log.Fatalf here: it calls os.Exit,
		// which skips defers and would abandon any in-flight ingest
		// queue / partial batch. Instead, log the error, drain the
		// ingest pipeline, and exit cleanly.
		log.Printf("[DOG] http server error: %v", err)
		in.Close()
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
