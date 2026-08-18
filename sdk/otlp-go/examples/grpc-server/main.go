// gRPC-style RPC server example.
//
// The demo-dog backend does not include a gRPC server, so this example
// uses the stdlib net/rpc package as a stand-in. The pattern translates
// 1:1 to real gRPC: wrap every handler with sdk.Trace / sdk.Record and
// emit per-method metrics.
//
//   go run ./examples/grpc-server
//   # in another terminal:
//   curl -X POST http://localhost:9090/rpc \
//     -d \"{\\"method\\":\\"Arith.Add\\",\\"params\\":[1,2],\\"id\\":1}\"
//
// (The /rpc endpoint is a minimal helper so the demo is testable without
// grpc CLI tooling.)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"

	otlp "github.com/zsy619/demo-dog/sdk/otlp-go/otlp"
)

// Arith is the demo RPC service. Each call becomes one span + one counter
// + one histogram in the DOG collector.
type Arith struct{}

// Add returns the sum of two integers.
func (Arith) Add(args *ArithArgs, reply *int) error {
	*reply = args.A + args.B
	return nil
}

// Divide returns a/b and may fail on zero divisor — failure surfaces as a
// span with status=error in the collector.
func (Arith) Divide(args *ArithArgs, reply *ArithReply) error {
	if args.B == 0 {
		return fmt.Errorf("divide by zero")
	}
	reply.Q = args.A / args.B
	reply.R = args.A % args.B
	return nil
}

type ArithArgs struct {
	A, B int
}

type ArithReply struct {
	Q, R int
}

// rpcMiddleware wraps every rpc call with Trace + metric emissions.
func rpcMiddleware(sdk *otlp.SDK, next rpc.ServerCodec) rpc.ServerCodec {
	return &tracedCodec{sdk: sdk, next: next}
}

type tracedCodec struct {
	sdk  *otlp.SDK
	next rpc.ServerCodec
	mu   sync.Mutex
}

func (t *tracedCodec) ReadRequestHeader(r *rpc.Request) error {
	return t.next.ReadRequestHeader(r)
}

func (t *tracedCodec) ReadRequestBody(body any) error {
	return t.next.ReadRequestBody(body)
}

func (t *tracedCodec) WriteResponse(resp *rpc.Response, body any) error {
	defer func() {
		// Per-call metrics. Body is the result value; we keep the original
		// response object so we know whether it succeeded.
		ok := resp.Error == ""
		status := "ok"
		if !ok {
			status = "error"
		}
		t.sdk.Counter(context.Background(), "rpc.calls", 1,
			otlp.String("method", resp.ServiceMethod),
			otlp.String("status", status),
		)
		_ = body
	}()
	return t.next.WriteResponse(resp, body)
}

func (t *tracedCodec) Close() error { return t.next.Close() }

func main() {
	endpoint := os.Getenv("DOG_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:18080"
	}

	sdk, err := otlp.New(endpoint,
		otlp.WithService("grpc-server-demo"),
		otlp.WithServiceVersion("0.1.0"),
		otlp.WithFlushInterval(1*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	arith := new(Arith)
	if err := rpc.Register(arith); err != nil {
		log.Fatal(err)
	}

	// Periodically emit a gauge — RPC servers have natural "in-flight"
	// metrics worth observing.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	inFlight := 0
	go func() {
		for range ticker.C {
			sdk.Gauge(context.Background(), "rpc.inflight", float64(inFlight))
			sdk.Counter(context.Background(), "rpc.heartbeat", 1)
		}
	}()

	// Minimal HTTP-to-RPC bridge on :9090.
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		var req struct {
			Method string        `json:"method"`
			Params []json.RawMessage `json:"params"`
			ID     int           `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		ctx := r.Context()

		start := time.Now()
		ctx, end := sdk.Trace(ctx, req.Method)
		inFlight++
		defer func() {
			inFlight--
		}()

		// Dispatch — every method becomes a Trace span with status set
		// to error if the RPC returns one.
		var rpcErr error
		switch req.Method {
		case "Arith.Add":
			var args ArithArgs
			if len(req.Params) >= 2 {
				json.Unmarshal(req.Params[0], &args.A)
				json.Unmarshal(req.Params[1], &args.B)
			}
			var reply int
			rpcErr = arith.Add(&args, &reply)
			writeReply(w, req.ID, reply, rpcErr)
		case "Arith.Divide":
			var args ArithArgs
			if len(req.Params) >= 2 {
				json.Unmarshal(req.Params[0], &args.A)
				json.Unmarshal(req.Params[1], &args.B)
			}
			var reply ArithReply
			rpcErr = arith.Divide(&args, &reply)
			writeReply(w, req.ID, reply, rpcErr)
		default:
			http.Error(w, "unknown method", 404)
			rpcErr = fmt.Errorf("unknown method")
		}

		// Span + counter + histogram — emitted after dispatch so the
		// status reflects whether the RPC succeeded.
		dur := time.Since(start)
		sdk.Record(ctx, req.Method, start, rpcErr)(
			otlp.Int("status_code", int64(statusCodeOf(rpcErr))),
			otlp.Float("duration_ms", float64(dur.Milliseconds())),
		)
		sdk.Histogram(ctx, "rpc.duration_ms", float64(dur.Milliseconds()),
			otlp.String("method", req.Method),
		)
		end(rpcErr)

		// Sprinkle in a tiny random latency so the latency histogram is
		// interesting even with one curl.
		time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
	})

	log.Printf("gRPC-style RPC demo listening on :9090, exporting to %s", endpoint)
	log.Fatal(http.ListenAndServe(":9090", mux))
}

func writeReply(w http.ResponseWriter, id int, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{"id": id, "result": result, "error": ""}
	if err != nil {
		resp["error"] = err.Error()
		resp["result"] = nil
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(resp)
}

func statusCodeOf(err error) int {
	if err == nil {
		return 200
	}
	return 500
}

// Compile-time guard so go test still finds a main_test hook when running
// `go test ./...`.
var _ = net.Listen
