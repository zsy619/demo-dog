// Package middleware 提供一个轻量的 HTTP 中间件链接器。
// 它暴露链构造器与一组常用中间件：恢复、日志、限流、缓存控制。
package middleware

import (
	"context"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// Handler 是中间件链接后的处理函数。
type Handler func(http.Handler) http.Handler

// Chain 按从外到内顺序应用中间件。
func Chain(h http.Handler, mws ...Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// Recovre 在 panic 时恢复并返回 500。
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "internal", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder 包装 ResponseWriter 记录状态码。
type statusRecorder struct {
	http.ResponseWriter
	status atomic.Int64
	wrote  atomic.Bool
}

func (s *statusRecorder) WriteHeader(c int) {
	s.status.Store(int64(c))
	s.wrote.Store(true)
	s.ResponseWriter.WriteHeader(c)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote.Load() {
		s.status.Store(int64(http.StatusOK))
		s.wrote.Store(true)
	}
	return s.ResponseWriter.Write(b)
}

// Logger 输出每个请求的方法、路径、状态码与耗时。
func Logger(out *log.Logger) Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if out != nil {
				out.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status.Load(), time.Since(start))
			}
		})
	}
}

// Timeout 限制每个请求的处理时长，超时返回 504。
func Timeout(d time.Duration) Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()
			select {
			case <-done:
			case <-ctx.Done():
				http.Error(w, "timeout", http.StatusGatewayTimeout)
			}
		})
	}
}

// CacheControl 设置给定 Cache-Control 头。
func CacheControl(value string) Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", value)
			next.ServeHTTP(w, r)
		})
	}
}

// CORS 设置基本的 CORS 响应头。
func CORS(origin string) Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
