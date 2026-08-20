// Self-tracing：一个中间件，将每个请求包裹为一个 span 并 POST 回
// 本地的 /api/ingest/otlp 端点。这使得单个
// 采集器可以在同一进程中无需 SDK 的情况下
// 绘制自身的时延图。
// 
// 实现刻意保持精简：每个请求一个 span，
// best-effort POST，没有采样、没有排队。产生
// 自身遥测数据的采集器就是一个 bootstrapper。

package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	selfTraceMu       sync.Mutex
	selfTraceEnabled  bool
	selfTraceLoopback string
)

var selfTraceSeq uint64

func (s *Server) selfTraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		selfTraceMu.Lock()
		enabled := selfTraceEnabled
		loopback := selfTraceLoopback
		selfTraceMu.Unlock()

		if !enabled {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		spanID := mintSpanID()
		traceID := mintTraceID()
		w.Header().Set("X-Dog-Trace-Id", traceID)
		w.Header().Set("X-Dog-Span-Id", spanID)

		// Wrap w so we can read the status code after the inner handler.
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		go func(method, path string, dur time.Duration, spanID, traceID string, status int) {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			body, _ := json.Marshal(map[string]any{
				"tenant_id": "self",
				"spans": []map[string]any{{
					"trace_id":    traceID,
					"span_id":     spanID,
					"service":     "dog-collector",
					"name":        "http.request",
					"start":       start.UTC().Format(time.RFC3339Nano),
					"duration_ms": dur.Milliseconds(),
					"status":      statusString(status),
					"attributes": map[string]string{
						"http.method": method,
						"http.path":   path,
						"http.status_code": strconv.Itoa(status),
					},
				}},
			})
			req, err := http.NewRequestWithContext(ctx, http.MethodPost,
				loopback+"/api/ingest/otlp", bytes.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
		}(r.Method, r.URL.Path, dur, spanID, traceID, sw.Status())
	})
}

func statusString(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "ok"
	default:
		return "error"
	}
}

func mintTraceID() string {
	seq := atomic.AddUint64(&selfTraceSeq, 1)
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + encodeSeq(seq)
}

func mintSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func encodeSeq(seq uint64) string {
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(seq >> (i * 8))
	}
	return hex.EncodeToString(b[:])
}

// statusWriter 包装 ResponseWriter，以便外层中间件能够在内部处理器
// 运行之后读取状态码。
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.wrote {
		sw.status = code
		sw.wrote = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if !sw.wrote {
		sw.status = http.StatusOK
		sw.wrote = true
	}
	return sw.ResponseWriter.Write(b)
}

func (sw *statusWriter) Status() int {
	if !sw.wrote {
		return http.StatusOK
	}
	return sw.status
}
