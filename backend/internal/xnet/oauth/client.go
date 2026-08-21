// Package oauth OAuth 授权码流程:state 校验 + token 交换。
//
// 文件职责拆分:
//   - client.go  Client + Server 类型 + 注册/查找
//   - token.go   TokenResponse + IssueClientCredentials + VerifyToken
//   - crypto.go  JWT 签名/校验 + 内部辅助
package oauth

import (
	"sync"
	"time"
)

// Client 是一个 OAuth2 客户端(机器对机器)。
type Client struct {
	ID         string        // 客户端 ID
	Secret     []byte        // 客户端密钥
	Scopes     []string      // 允许的作用域
	Tenant     string        // 所属租户
	AccessTTL  time.Duration // 访问令牌 TTL
	RefreshTTL time.Duration // 刷新令牌 TTL
	now        func() time.Time // 时钟源
}

// Server 是 OAuth2 令牌签发器。
type Server struct {
	mu      sync.RWMutex       // 保护 clients
	clients map[string]*Client  // 已注册客户端
	issuer  string              // 签发方 URL
	secret  []byte              // HMAC 签名密钥
	now     func() time.Time    // 时钟源
}

// New 使用指定的 issuer URL 与签名密钥创建一个 Server。
func New(issuer, secret string) *Server {
	return &Server{
		clients: make(map[string]*Client),
		issuer:  issuer,
		secret:  []byte(secret),
		now:     time.Now,
	}
}

// WithTime 覆盖测试所用的时间源。
func (s *Server) WithTime(now func() time.Time) *Server {
	s.now = now
	return s
}

// Register 添加一个客户端;AccessTTL/RefreshTTL <= 0 自动填充默认。
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

// ClientLookup 按 ID 查找客户端。
func (s *Server) ClientLookup(id string) (*Client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.clients[id]
	return c, ok
}
