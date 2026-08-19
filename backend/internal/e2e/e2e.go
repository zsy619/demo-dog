// Package e2e 提供一个轻量的端到端测试辅助工具。
// 它包装 net/http/httptest.Server，允许按步骤运行并断言状态。
package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Client 是一个线程安全的端到端测试客户端。
type Client struct {
	mu     sync.Mutex
	server *httptest.Server
	http   *http.Client
}

// New 创建一个 Client，绑定到 httptest.Server。
func New(server *httptest.Server) *Client {
	return &Client{
		server: server,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
}

// URL 返回测试服务器 URL。
func (c *Client) URL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.server == nil {
		return ""
	}
	return c.server.URL
}

// Close 关闭服务器。
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.server != nil {
		c.server.Close()
		c.server = nil
	}
}

// Response 是断言友好的 HTTP 响应包装。
type Response struct {
	Status   int
	Body     []byte
	Headers  http.Header
	Elapsed  time.Duration
}

// Do 执行一次请求并返回 Response。
func (c *Client) Do(method, path string, body []byte, headers map[string]string) (*Response, error) {
	c.mu.Lock()
	if c.server == nil {
		c.mu.Unlock()
		return nil, errors.New("e2e: 服务器已关闭")
	}
	url := c.server.URL + path
	c.mu.Unlock()
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return &Response{
		Status:  resp.StatusCode,
		Body:    b,
		Headers: resp.Header,
		Elapsed: time.Since(start),
	}, nil
}

// Get 便捷 GET 封装。
func (c *Client) Get(path string) (*Response, error) {
	return c.Do(http.MethodGet, path, nil, nil)
}

// PostJSON 发送 JSON POST 请求。
func (c *Client) PostJSON(path string, payload any) (*Response, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.Do(http.MethodPost, path, b, map[string]string{"Content-Type": "application/json"})
}

// ExpectStatus 断言响应码。
func (r *Response) ExpectStatus(t *testing.T, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("状态码不匹配: 期望 %d, 实际 %d, body=%s", want, r.Status, string(r.Body))
	}
}

// ExpectBody 断言响应体按子串匹配。
func (r *Response) ExpectBody(t *testing.T, substr string) {
	t.Helper()
	if !bytes.Contains(r.Body, []byte(substr)) {
		t.Fatalf("body 不包含 %q: %s", substr, string(r.Body))
	}
}

// DecodeJSON 解析响应体为 JSON。
func (r *Response) DecodeJSON(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		t.Fatalf("json 解码失败: %v: %s", err, string(r.Body))
	}
}
