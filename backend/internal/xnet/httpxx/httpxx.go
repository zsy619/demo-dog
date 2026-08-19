// Package httpxx 提供 net/http 客户端的简单封装：
// 默认 timeout + 统一 JSON 序列化 + 状态码包装。
package httpxx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// Client 是封装后的 HTTP 客户端。
type Client struct {
	c           *http.Client
	baseURL     string
	headers     map[string]string
	timeout     time.Duration
	dialTimeout time.Duration
}

// New 创建一个默认 10s 超时的客户端。
func New() *Client {
	return &Client{
		c:           &http.Client{Timeout: 10 * time.Second},
		headers:     make(map[string]string),
		timeout:     10 * time.Second,
		dialTimeout: 5 * time.Second,
	}
}

// SetBaseURL 设置基础 URL。
func (c *Client) SetBaseURL(u string) { c.baseURL = u }

// SetTimeout 设置总超时。
func (c *Client) SetTimeout(d time.Duration) { c.timeout = d; c.c.Timeout = d }

// SetHeader 设置默认 Header。
func (c *Client) SetHeader(k, v string) { c.headers[k] = v }

// SetTransport 设置自定义 Transport。
func (c *Client) SetTransport(t *http.Transport) {
	c.c.Transport = t
}

// Get 发起 GET 请求。
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Post 发起 POST 请求，body 自动 JSON 序列化。
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewReader(b)
	}
	return c.do(ctx, http.MethodPost, path, buf)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.c.Do(req)
}

// DoJSON 把响应解析为结构体。
func DoJSON(r *http.Response, out any) error {
	if r == nil {
		return errors.New("httpxx: nil response")
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		b, _ := io.ReadAll(r.Body)
		return errors.New(string(b))
	}
	return json.NewDecoder(r.Body).Decode(out)
}

// DialFunc 是可选 dialer 注入点。
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)
