// Package reqlog 提供 HTTP 请求日志记录辅助。
package reqlog

import (
	"net/http"
	"sync/atomic"
	"time"
)

// Recorder 是访问日志记录器。
type Recorder struct {
	seq  atomic.Uint64
	w    http.ResponseWriter
	st   atomic.Int64
	size atomic.Int64
	start time.Time
}

// Wrap 包装一个 ResponseWriter 用于追踪状态码与字节数。
func Wrap(w http.ResponseWriter) *Recorder {
	return &Recorder{w: w, start: time.Now()}
}

// Header 实现 http.ResponseWriter。
func (r *Recorder) Header() http.Header {
	return r.w.Header()
}

// Write 写入数据并累加计数。
func (r *Recorder) Write(p []byte) (int, error) {
	n, err := r.w.Write(p)
	r.size.Add(int64(n))
	return n, err
}

// WriteHeader 写入状态码。
func (r *Recorder) WriteHeader(code int) {
	r.st.Store(int64(code))
	r.w.WriteHeader(code)
}

// Status 返回捕获的状态码（默认 200）。
func (r *Recorder) Status() int {
	v := r.st.Load()
	if v == 0 {
		return 200
	}
	return int(v)
}

// Size 返回写入的字节数。
func (r *Recorder) Size() int64 { return r.size.Load() }

// Duration 返回从 Wrap 起经过的时长。
func (r *Recorder) Duration() time.Duration { return time.Since(r.start) }

// ID 返回本次请求的单调递增 ID。
func (r *Recorder) ID() uint64 { return r.seq.Add(1) }

// Log 是请求完成后的标准日志条目。
type Log struct {
	ID       uint64        `json:"id"`
	Method   string        `json:"method"`
	Path     string        `json:"path"`
	Status   int           `json:"status"`
	Size     int64         `json:"size"`
	Latency  time.Duration `json:"latency"`
	Remote   string        `json:"remote"`
}

// Snapshot 返回当前 Log。
func (r *Recorder) Snapshot(req *http.Request) Log {
	return Log{
		ID:      r.ID(),
		Method:  req.Method,
		Path:    req.URL.Path,
		Status:  r.Status(),
		Size:    r.Size(),
		Latency: r.Duration(),
		Remote:  req.RemoteAddr,
	}
}
