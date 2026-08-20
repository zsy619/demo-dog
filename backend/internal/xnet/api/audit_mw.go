package api

import (
	"bufio"
	"net"
	"net/http"
	"time"
)

// auditResponseWriter 捕获状态码和写入的字节数,以便
// 审计中间件可以将它们包含在日志行中。
type auditResponseWriter struct {
	w          http.ResponseWriter
	status     int
	bytes      int64
	wroteHead  bool
}

func (a *auditResponseWriter) Header() http.Header { return a.w.Header() }

func (a *auditResponseWriter) WriteHeader(code int) {
	if a.wroteHead {
		return
	}
	a.status = code
	a.wroteHead = true
	a.w.WriteHeader(code)
}

func (a *auditResponseWriter) Write(p []byte) (int, error) {
	if !a.wroteHead {
		a.WriteHeader(http.StatusOK)
	}
	n, err := a.w.Write(p)
	a.bytes += int64(n)
	return n, err
}

// Hijack 允许下游处理器(尤其是 WebSocket 升级)
// 接管连接。审计中间件无需
// 包装被劫持的流。
func (a *auditResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := a.w.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Flush 在底层 writer 支持时进行代理转发。代码库中的
// 一些 SSE 处理器依赖 Flusher 来推送事件。
func (a *auditResponseWriter) Flush() {
	if f, ok := a.w.(http.Flusher); ok {
		f.Flush()
	}
}

// AuditMiddleware returns an http middleware that records one
// AuditEvent per request to `sink`. Only write operations (POST,
// PUT, DELETE) are recorded by default; readers can opt-in via
// `recordReads`.
func AuditMiddleware(sink *AuditLog, recordReads bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !recordReads && r.Method == http.MethodGet {
				next.ServeHTTP(w, r)
				return
			}
			// Wrap to capture the eventual status code + bytes out.
			arw := &auditResponseWriter{w: w, status: http.StatusOK}
			next.ServeHTTP(arw, r)

			sink.Append(AuditEvent{
				Timestamp: time.Now(),
				Method:    r.Method,
				Path:      r.URL.Path,
				KeyLabel:  r.Header.Get("X-Dog-Key-Label"),
				Role:      r.Header.Get("X-Dog-Role"),
				Tenant:    r.Header.Get("X-Tenant-Id"),
				Status:    arw.status,
				BytesIn:   r.ContentLength,
				BytesOut:  arw.bytes,
				RemoteIP:  clientIP(r),
				UserAgent: r.Header.Get("User-Agent"),
			})
		})
	}
}
