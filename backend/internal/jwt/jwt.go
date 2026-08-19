package jwt

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Algorithm is the signing algorithm.
type Algorithm string

const (
	HS256 Algorithm = "HS256"
)

// Key carries the secret + kid for one rotation entry.
type Key struct {
	KID     string
	Secret  []byte
	Alg     Algorithm
	Created time.Time
}

// Verifier holds a ring of keys by kid, with the current key
// always at index 0.
type Verifier struct {
	mu        sync.RWMutex
	keys      map[string]*Key
	order     []string
	clockSkew time.Duration
	now       func() time.Time
}

// New creates an empty Verifier.
func New(clockSkew time.Duration) *Verifier {
	if clockSkew <= 0 {
		clockSkew = 30 * time.Second
	}
	return &Verifier{
		keys:      make(map[string]*Key),
		clockSkew: clockSkew,
		now:       time.Now,
	}
}

// WithTime overrides the time source for tests.
func (v *Verifier) WithTime(now func() time.Time) *Verifier {
	v.now = now
	return v
}

// Add introduces a key. New keys go to the front of the order.
func (v *Verifier) Add(k *Key) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys[k.KID] = k
	v.order = append([]string{k.KID}, v.order...)
}

// Remove drops a key by kid.
func (v *Verifier) Remove(kid string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.keys, kid)
	out := v.order[:0]
	for _, k := range v.order {
		if k != kid {
			out = append(out, k)
		}
	}
	v.order = out
}

// ErrNoKey is returned when no key matches the kid.
var ErrNoKey = errors.New("no key")

// ErrBadToken is returned for malformed tokens.
var ErrBadToken = errors.New("bad token")

// ErrExpired is returned when the token is past its expiry.
var ErrExpired = errors.New("expired")

// Verify parses and validates a token, returning the kid.
func (v *Verifier) Verify(token string) (map[string]any, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, "", ErrBadToken
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, "", ErrBadToken
	}
	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, "", ErrBadToken
	}
	kid, _ := header["kid"].(string)
	v.mu.RLock()
	k, ok := v.keys[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, "", ErrNoKey
	}
	mac := hmac.New(sha256.New, k.Secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if macStr := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)); macStr != parts[2] {
		return nil, "", ErrBadToken
	}
	bodyBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", ErrBadToken
	}
	var claims map[string]any
	if err := json.Unmarshal(bodyBytes, &claims); err != nil {
		return nil, "", ErrBadToken
	}
	if exp, ok := claims["exp"].(float64); ok {
		if v.now().Unix() > int64(exp) {
			return claims, kid, ErrExpired
		}
	}
	if nbf, ok := claims["nbf"].(float64); ok {
		if v.now().Unix() < int64(nbf) {
			return claims, kid, ErrBadToken
		}
	}
	return claims, kid, nil
}

type ctxKey int

const (
	claimsKey ctxKey = iota
	kidKey
)

type claimsCtx struct {
	ctx    context.Context
	claims map[string]any
	kid    string
}

func (c *claimsCtx) Value(key any) any {
	switch key {
	case claimsKey:
		return c.claims
	case kidKey:
		return c.kid
	}
	return c.ctx.Value(key)
}

func (c *claimsCtx) Deadline() (time.Time, bool)       { return c.ctx.Deadline() }
func (c *claimsCtx) Done() <-chan struct{}             { return c.ctx.Done() }
func (c *claimsCtx) Err() error                        { return c.ctx.Err() }

// WithClaims wraps a context.Context with claims + kid.
func WithClaims(ctx context.Context, claims map[string]any, kid string) context.Context {
	return &claimsCtx{ctx: ctx, claims: claims, kid: kid}
}

// FromContext returns the claims map + kid injected by the
// middleware.
func FromContext(ctx context.Context) (map[string]any, string, bool) {
	c, ok := ctx.Value(claimsKey).(map[string]any)
	if !ok {
		return nil, "", false
	}
	kid, _ := ctx.Value(kidKey).(string)
	return c, kid, true
}

// Sign issues a HS256 token for the given claims using the
// given key.
func Sign(claims map[string]any, k *Key) (string, error) {
	header := map[string]any{"alg": "HS256", "typ": "JWT", "kid": k.KID}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	hp := base64.RawURLEncoding.EncodeToString(hb)
	cp := base64.RawURLEncoding.EncodeToString(cb)
	mac := hmac.New(sha256.New, k.Secret)
	mac.Write([]byte(hp + "." + cp))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hp + "." + cp + "." + sig, nil
}

// Middleware returns the http.Handler that requires a valid
// bearer token + injects claims into the request context.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		tok := strings.TrimPrefix(auth, "Bearer ")
		claims, kid, err := v.Verify(tok)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := WithClaims(r.Context(), claims, kid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
