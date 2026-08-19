// Package oauth2 提供 OAuth2 授权码流程的轻量级辅助：
// 生成 state、PKCE verifier/challenge、构造授权 URL 与 token URL。
package oauth2

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
)

// Config 是 OAuth2 客户端配置。
type Config struct {
	ClientID    string
	AuthURL     string
	TokenURL    string
	RedirectURL string
	Scopes      []string
}

// Verifier 包含 PKCE verifier 与 challenge。
type Verifier struct {
	CodeVerifier  string
	CodeChallenge string
	Method        string // S256 或 plain
}

// NewVerifier 生成一个 S256 PKCE verifier。
func NewVerifier() (*Verifier, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	v := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(v))
	ch := base64.RawURLEncoding.EncodeToString(sum[:])
	return &Verifier{CodeVerifier: v, CodeChallenge: ch, Method: "S256"}, nil
}

// NewState 生成一个随机 state 串。
func NewState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// AuthorizeURL 拼接授权 URL（含 state、pkce challenge）。
func AuthorizeURL(cfg Config, state string, v *Verifier) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURL)
	if len(cfg.Scopes) > 0 {
		q.Set("scope", joinScopes(cfg.Scopes))
	}
	if state != "" {
		q.Set("state", state)
	}
	if v != nil {
		q.Set("code_challenge", v.CodeChallenge)
		q.Set("code_challenge_method", v.Method)
	}
	return cfg.AuthURL + "?" + q.Encode()
}

// TokenRequest 表示一个 token 端点请求的参数。
type TokenRequest struct {
	GrantType    string
	Code         string
	CodeVerifier string
	RedirectURI  string
	RefreshToken string
}

// Form 把请求编码为 application/x-www-form-urlencoded 形式。
func (t TokenRequest) Form(clientID string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("grant_type", t.GrantType)
	switch t.GrantType {
	case "authorization_code":
		q.Set("code", t.Code)
		q.Set("redirect_uri", t.RedirectURI)
		if t.CodeVerifier != "" {
			q.Set("code_verifier", t.CodeVerifier)
		}
	case "refresh_token":
		q.Set("refresh_token", t.RefreshToken)
	}
	return q.Encode()
}

// Errors
var (
	ErrEmptyCode = errors.New("oauth2: code 为空")
	ErrBadState  = errors.New("oauth2: state 不匹配")
)

// ValidateCallback 校验回调 code 与 state。
func ValidateCallback(code, expected, gotState string) error {
	if code == "" {
		return ErrEmptyCode
	}
	if expected != "" && expected != gotState {
		return ErrBadState
	}
	return nil
}

// JoinScopes 拼接 scope。
func joinScopes(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += " "
		}
		out += x
	}
	return out
}

// Sanitize 用 fmt.Sprintf 简洁打印 config。
func (c Config) Sanitize() string {
	return fmt.Sprintf("OAuthConfig{client=%s, redirect=%s, scopes=%v}", c.ClientID, c.RedirectURL, c.Scopes)
}
