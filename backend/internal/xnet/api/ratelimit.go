package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	now    func() time.Time
	bucket map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(rate, burst float64) *RateLimiter {
	if rate < 0 {
		rate = 0
	}
	if burst <= 0 {
		burst = rate
	}
	return &RateLimiter{
		rate:   rate,
		burst:  burst,
		now:    time.Now,
		bucket: make(map[string]*bucket),
	}
}

func (r *RateLimiter) Allow(ip string) bool {
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.bucket[ip]
	if !ok {
		b = &bucket{tokens: r.burst, last: now}
		r.bucket[ip] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * r.rate
	if b.tokens > r.burst {
		b.tokens = r.burst
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens -= 1
		return true
	}
	return false
}

func (r *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ip := clientIP(req)
			if !r.Allow(ip) {
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests, errRateLimited)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' || xff[i] == ' ' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

var errRateLimited = &rateLimitError{}

type rateLimitError struct{}

func (e *rateLimitError) Error() string { return "rate limit exceeded" }


// AllowByKey 是 Allow 的 per-API-key 变体。返回 (allowed,
// remaining, retryAfter)。供对调用方进行鉴权并希望
// 比 per-IP 变体进行更细粒度限流的中间件使用。
func (r *RateLimiter) AllowByKey(key string) (bool, int, time.Duration) {
	if key == "" {
		return true, 0, 0
	}
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.bucket[key]
	if !ok {
		b = &bucket{tokens: r.burst, last: now}
		r.bucket[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * r.rate
	if b.tokens > r.burst {
		b.tokens = r.burst
	}
	b.last = now
	if b.tokens >= 1 {
		b.tokens -= 1
		return true, int(b.tokens), 0
	}
	retry := time.Duration((1 - b.tokens) / r.rate * float64(time.Second))
	return false, 0, retry
}

// Stats 返回 /api/health 的当前限流器状态。
type RateLimiterStats struct {
	Keys  int     `json:"keys"`
	Rate  float64 `json:"rate"`
	Burst float64 `json:"burst"`
}

func (r *RateLimiter) Stats() RateLimiterStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return RateLimiterStats{
		Keys:  len(r.bucket),
		Rate:  r.rate,
		Burst: r.burst,
	}
}

// KeyedMiddleware 用 per-API-key 限流包裹一个处理器。
// 传入一个将请求解析为 key 的函数（通常是
// Authorization bearer token，或匿名路径下的 X-Dog-Tenant）。当桶为空时，
// 处理器会返回 429，并附带根据实际补充速率
// 计算得出的 Retry-After 头部。
func (r *RateLimiter) KeyedMiddleware(resolveKey func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			key := resolveKey(req)
			if key == "" {
				next.ServeHTTP(w, req)
				return
			}
			allowed, remaining, retry := r.AllowByKey(key)
			if !allowed {
				secs := int(retry.Seconds())
				if secs < 1 { secs = 1 }
				w.Header().Set("Retry-After", strconvItoa(secs))
				w.Header().Set("X-RateLimit-Remaining", "0")
				writeError(w, http.StatusTooManyRequests, errRateLimited)
				return
			}
			w.Header().Set("X-RateLimit-Remaining", strconvItoa(remaining))
			next.ServeHTTP(w, req)
		})
	}
}

// strconvItoa 是一个本地的 Itoa 实现，避免将 strconv 拉入
// 已有文件的 import 列表。
func strconvItoa(n int) string {
	if n == 0 { return "0" }
	neg := false
	if n < 0 { neg = true; n = -n }
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg { i--; b[i] = '-' }
	return string(b[i:])
}
