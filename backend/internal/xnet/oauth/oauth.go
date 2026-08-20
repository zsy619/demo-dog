// Package oauth OAuth 授权码流程：state 校验 + token 交换。
package oauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Client is one OAuth2 client (machine-to-machine).
type Client struct {
	ID           string
	Secret       []byte
	Scopes       []string
	Tenant       string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	now          func() time.Time
}

// Server is the OAuth2 token issuer.
type Server struct {
	mu       sync.RWMutex
	clients  map[string]*Client
	issuer   string
	secret   []byte
	now      func() time.Time
}

// New creates a Server with the given issuer URL + signing
// secret.
func New(issuer, secret string) *Server {
	return &Server{
		clients: make(map[string]*Client),
		issuer: issuer,
		secret: []byte(secret),
		now: time.Now,
	}
}

// WithTime overrides the time source for tests.
func (s *Server) WithTime(now func() time.Time) *Server {
	s.now = now
	return s
}

// Register adds a client.
func (s *Server) Register(c *Client) {
	if c.AccessTTL <= 0 {
		c.AccessTTL = 1 * time.Hour
	}
	if c.RefreshTTL <= 0 {
		c.RefreshTTL = 24 * time.Hour
	}
	c.now = s.now
	s.mu.Lock()
	s.clients[c.ID] = c
	s.mu.Unlock()
}

// ClientLookup fetches a client by ID.
func (s *Server) ClientLookup(id string) (*Client, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.clients[id]
	return c, ok
}

// ErrInvalidClient is returned when the client_id / secret
// pair is wrong.
var ErrInvalidClient = errors.New("invalid client")

// ErrInvalidScope is returned when the requested scope is
// not allowed for the client.
var ErrInvalidScope = errors.New("invalid scope")

// TokenResponse is the OAuth2 token JSON response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// IssueClientCredentials implements the client_credentials
// grant. Returns the token response on success.
func (s *Server) IssueClientCredentials(clientID, clientSecret string, scopes []string) (*TokenResponse, error) {
	s.mu.RLock()
	c, ok := s.clients[clientID]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrInvalidClient
	}
	if !hmac.Equal([]byte(clientSecret), c.Secret) {
		return nil, ErrInvalidClient
	}
	if len(scopes) > 0 {
		allowed := make(map[string]bool, len(c.Scopes))
		for _, sc := range c.Scopes {
			allowed[sc] = true
		}
		for _, sc := range scopes {
			if !allowed[sc] {
				return nil, ErrInvalidScope
			}
		}
	}
	now := s.now()
	exp := now.Add(c.AccessTTL)
	access, err := s.signToken(map[string]any{
		"sub": clientID,
		"tid": c.Tenant,
		"scp": scopes,
		"iss": s.issuer,
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"typ": "access",
	})
	if err != nil {
		return nil, err
	}
	refresh, err := newOpaque()
	if err != nil {
		return nil, err
	}
	return &TokenResponse{
		AccessToken: access,
		TokenType: "Bearer",
		ExpiresIn: int(c.AccessTTL.Seconds()),
		RefreshToken: refresh,
		Scope: joinScopes(scopes),
	}, nil
}

// VerifyToken returns the token claims if signature + expiry
// are valid.
func (s *Server) VerifyToken(token string) (map[string]any, error) {
	claims, err := s.verifyToken(token)
	if err != nil {
		return nil, err
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, errors.New("missing exp")
	}
	if s.now().Unix() > int64(exp) {
		return nil, errors.New("token expired")
	}
	return claims, nil
}

func (s *Server) signToken(claims map[string]any) (string, error) {
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
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
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(hp + "." + cp))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hp + "." + cp + "." + sig, nil
}

func (s *Server) verifyToken(token string) (map[string]any, error) {
	parts := splitN(token, '.', 3)
	if len(parts) != 3 {
		return nil, errors.New("bad token")
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(want)) {
		return nil, errors.New("bad signature")
	}
	cb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(cb, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func newOpaque() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	out := ""
	for i, sc := range scopes {
		if i > 0 {
			out += " "
		}
		out += sc
	}
	return out
}

func splitN(s string, sep byte, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i +1
			if len(out) == n-1 {
				break
			}
		}
	}
	out = append(out, s[start:])
	return out
}

// MintToken returns the raw access token string for tests.
func MintToken(s *Server, claims map[string]any) (string, error) {
	return s.signToken(claims)
}

// MustDecode decodes the access token from a TokenResponse
// into its claims map (test helper).
func (s *Server) MustDecode(token string) (map[string]any, error) {
	return s.verifyToken(token)
}

// String returns a printable form for errors.
func String(b []byte) string { return fmt.Sprintf("%s", b) }
