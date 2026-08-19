package iplimit

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// bucket holds the per-IP state.
type bucket struct {
	tokens    float64
	lastRefil time.Time
}

// Limiter enforces a token-bucket rate limit per IP address.
// The bucket fills at `rate` requests per second up to `burst`
// burst capacity.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64
	burst    float64
	now      func() time.Time
	rejected atomic.Uint64
	accepted atomic.Uint64
	cleaned  atomic.Uint64
}

// New creates a Limiter. Rate is requests/second; burst is
// the maximum bucket size.
func New(rate, burst float64) *Limiter {
	if rate <= 0 {
		rate = 10
	}
	if burst <= 0 {
		burst = rate * 2
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		now:     time.Now,
	}
}

// WithTime overrides the time source for tests.
func (l *Limiter) WithTime(now func() time.Time) *Limiter {
	l.now = now
	return l
}

// Allow consumes one token for ip. Returns true if allowed.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[ip]
	if !ok {
		b = &bucket{tokens: l.burst, lastRefil: now}
		l.buckets[ip] = b
	}
	elapsed := now.Sub(b.lastRefil).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.lastRefil = now
	}
	if b.tokens >= 1 {
		b.tokens--
		l.accepted.Add(1)
		return true
	}
	l.rejected.Add(1)
	return false
}

// Cleanup removes buckets older than maxIdle. Returns the
// number removed. Call from a timer.
func (l *Limiter) Cleanup(maxIdle time.Duration) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	n := 0
	for ip, b := range l.buckets {
		if now.Sub(b.lastRefil) > maxIdle {
			delete(l.buckets, ip)
			n++
		}
	}
	l.cleaned.Add(uint64(n))
	return n
}

// Stats returns the counters.
type Stats struct {
	Rate     float64 `json:"rate"`
	Burst    float64 `json:"burst"`
	IPs      int     `json:"ips"`
	Accepted uint64  `json:"accepted"`
	Rejected uint64  `json:"rejected"`
	Cleaned  uint64  `json:"cleaned"`
}

// Stats returns the snapshot.
func (l *Limiter) Stats() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return Stats{
		Rate: l.rate, Burst: l.burst,
		IPs: len(l.buckets),
		Accepted: l.accepted.Load(),
		Rejected: l.rejected.Load(),
		Cleaned: l.cleaned.Load(),
	}
}

// Middleware returns the http.Handler that enforces the rate
// limit. IP is read from X-Forwarded-For or RemoteAddr.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !l.Allow(ip) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("X-RateLimit-Limit", strconv.FormatFloat(l.rate, 'f', 0, 64))
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
