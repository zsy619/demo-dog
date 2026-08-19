// Package httpx 提供一个简单、带超时与重试的 HTTP 客户端封装。
package httpx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// Client 是一个简单包装的 HTTP 客户端。
type Client struct {
	httpc    *http.Client
	retry    int
	backoff  time.Duration
	userAgen string
}

// Config 是构造参数。
type Config struct {
	Timeout  time.Duration
	Retry    int
	Backoff  time.Duration
	UserAgen string
}

// New 创建一个 Client。
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Retry < 0 {
		cfg.Retry = 0
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 100 * time.Millisecond
	}
	if cfg.UserAgen == "" {
		cfg.UserAgen = "demo-dog/1.0"
	}
	return &Client{
		httpc:    &http.Client{Timeout: cfg.Timeout},
		retry:    cfg.Retry,
		backoff:  cfg.Backoff,
		userAgen: cfg.UserAgen,
	}
}

// Response 是单次调用的结果。
type Response struct {
	Status  int
	Body    []byte
	Headers http.Header
	Latency time.Duration
}

// ErrStatus 在状态码非 OK 时返回。
var ErrStatus = errors.New("httpx: 非 2xx 状态码")

// Do 执行一次请求。当返回 5xx 或传输错误时按 retry 次数重试。
func (c *Client) Do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*Response, error) {
	var last error
	for attempt := 0; attempt <= c.retry; attempt++ {
		resp, err := c.do(ctx, method, url, body, headers)
		if err != nil {
			last = err
			time.Sleep(time.Duration(attempt+1) * c.backoff)
			continue
		}
		if resp.Status >= 500 && attempt < c.retry {
			last = ErrStatus
			time.Sleep(time.Duration(attempt+1) * c.backoff)
			continue
		}
		return resp, nil
	}
	return nil, last
}

func (c *Client) do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*Response, error) {
	var br io.Reader
	if body != nil {
		br = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, br)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgen)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return &Response{
		Status:  resp.StatusCode,
		Body:    b,
		Headers: resp.Header,
		Latency: time.Since(start),
	}, nil
}

// Get 便捷 GET 包装。
func (c *Client) Get(ctx context.Context, url string) (*Response, error) {
	return c.Do(ctx, http.MethodGet, url, nil, nil)
}

// Post 便捷 POST 包装。
func (c *Client) Post(ctx context.Context, url string, body []byte, contentType string) (*Response, error) {
	return c.Do(ctx, http.MethodPost, url, body, map[string]string{"Content-Type": contentType})
}

// ExpectStatus 断言响应是 2xx。
func (r *Response) ExpectStatus() error {
	if r.Status < 200 || r.Status >= 300 {
		return ErrStatus
	}
	return nil
}
