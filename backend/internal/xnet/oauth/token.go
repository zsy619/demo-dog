package oauth

// token.go:TokenResponse + IssueClientCredentials + VerifyToken + 测试辅助。

import (
	"crypto/hmac"
	"errors"
)

// ErrInvalidClient 在 client_id / secret 错误时返回。
var ErrInvalidClient = errors.New("invalid client")

// ErrInvalidScope 在请求的作用域对客户端不允许时返回。
var ErrInvalidScope = errors.New("invalid scope")

// TokenResponse 是 OAuth2 令牌的 JSON 响应。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// IssueClientCredentials 实现 client_credentials grant,成功时返回 token 响应。
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
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(c.AccessTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        joinScopes(scopes),
	}, nil
}

// VerifyToken 在签名与有效期均合法时返回令牌 claims。
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

// MintToken 返回原始访问令牌字符串,供测试使用。
func MintToken(s *Server, claims map[string]any) (string, error) {
	return s.signToken(claims)
}

// MustDecode 从令牌解码 claims(测试辅助函数)。
func (s *Server) MustDecode(token string) (map[string]any, error) {
	return s.verifyToken(token)
}
