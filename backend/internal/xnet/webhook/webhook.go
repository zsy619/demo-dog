// Package webhook Webhook 回调：异步发送 HTTP 回调并重试。
package webhook

// 出站 webhook 投递，支持 HMAC 签名 + 重试。
//
// Subscribers 注册一个 URL + secret + event filter；
// dispatcher 使用 HMAC-SHA256 对 payload 签名并 POST 它。
// Subscribers 注册一个 URL + secret + event filter；
// attempts; permanently failed deliveries are kept in the
// 会被保留在 dead-letter ring 中供运维人员检查。
//
// 设计上会接入 alert manager：当一条规则触发时，
// 会派发一个 webhook event，触发的 webhook ID
// 会记录在 AlertEvent 中。

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Event 是一个投递负载。
type Event struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Tenant    string            `json:"tenant"`
	Payload   map[string]string `json:"payload"`
}

// Subscriber 描述一个出站目标。
type Subscriber struct {
	ID         string
	URL        string
	Secret     string
	EventTypes []string // empty = all events
	MaxRetries int
	Timeout    time.Duration
	Now        func() time.Time
}

func (s *Subscriber) now() func() time.Time {
	if s.Now == nil {
		return time.Now
	}
	return s.Now
}

func (s *Subscriber) timeout() time.Duration {
	if s.Timeout <= 0 {
		return 5 * time.Second
	}
	return s.Timeout
}

func (s *Subscriber) maxRetries() int {
	if s.MaxRetries < 0 {
		return 0
	}
	if s.MaxRetries > 10 {
		return 10
	}
	return s.MaxRetries
}

// Accept 报告 subscriber 是否需要此事件类型。
func (s *Subscriber) Accept(eventType string) bool {
	if len(s.EventTypes) == 0 {
		return true
	}
	for _, t := range s.EventTypes {
		if t == eventType {
			return true
		}
	}
	return false
}

// Delivery是一次发送尝试。
type Delivery struct {
	EventID    string
	SubscriberID string
	Attempts   int
	Status     int    // HTTP status, 0 if no response
	Error      string // empty on success
	Latency    time.Duration
	LastTry    time.Time
}

// Success 报告投递最终是否成功。
func (d Delivery) Success() bool { return d.Error == "" }

// Dispatcher 持有订阅者与死信环形缓冲区。
type Dispatcher struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	dlq         []Delivery
	dlqCap      int
	client      *http.Client
	countSent   atomic.Int64
	countFail   atomic.Int64
	dlqHead     int
}

// NewDispatcher 返回一个 dispatcher。
func NewDispatcher(dlqCap int) *Dispatcher {
	if dlqCap <= 0 {
		dlqCap = 256
	}
	d := &Dispatcher{
		subscribers: make(map[string]*Subscriber),
		dlq:         make([]Delivery, 0, dlqCap),
		dlqCap:      dlqCap,
		client:      &http.Client{},
	}
	return d
}

// AddSubscriber 注册一个订阅者。
func (d *Dispatcher) AddSubscriber(s *Subscriber) error {
	if s.ID == "" || s.URL == "" {
		return errors.New("id and url required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers[s.ID] = s
	return nil
}

// RemoveSubscriber 注销一个订阅者。
func (d *Dispatcher) RemoveSubscriber(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.subscribers, id)
}

// Subscribers 返回当前的订阅者集合。
func (d *Dispatcher) Subscribers() []*Subscriber {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*Subscriber, 0, len(d.subscribers))
	for _, s := range d.subscribers {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Dispatch 将一个 event 投递到每一个接受它的 subscriber。该函数返回投递列表。
// it. The function returns the list of deliveries.
func (d *Dispatcher) Dispatch(ev Event) []Delivery {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = d.now()()
	}
	if ev.ID == "" {
		ev.ID = fmt.Sprintf("evt-%d", ev.Timestamp.UnixNano())
	}
	d.mu.RLock()
	targets := make([]*Subscriber, 0, len(d.subscribers))
	for _, s := range d.subscribers {
		if s.Accept(ev.Type) {
			targets = append(targets, s)
		}
	}
	d.mu.RUnlock()
	out := make([]Delivery, 0, len(targets))
	for _, s := range targets {
		del := d.deliver(ev, s)
		out = append(out, del)
	}
	return out
}

func (d *Dispatcher) now() func() time.Time { return time.Now }

func (d *Dispatcher) deliver(ev Event, s *Subscriber) Delivery {
	del := Delivery{EventID: ev.ID, SubscriberID: s.ID}
	body, err := json.Marshal(ev)
	if err != nil {
		del.Error = fmt.Sprintf("marshal: %v", err)
		d.recordDLQ(del)
		return del
	}
	sig := Sign(body, s.Secret)
	max := s.maxRetries() + 1
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		del.Attempts = attempt
		del.LastTry = s.now()()
		start := s.now()()
		status, err := d.post(s, body, sig, ev.ID)
		del.Latency = s.now()().Sub(start)
		if err == nil && status >= 200 && status < 300 {
			del.Status = status
			d.countSent.Add(1)
			return del
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("status %d", status)
			del.Status = status
		}
		if attempt < max {
			time.Sleep(backoff(attempt))
		}
	}
	del.Error = lastErr.Error()
	d.countFail.Add(1)
	d.recordDLQ(del)
	return del
}

// backoff 返回第 n 次重试前的延迟（从 1 开始计数）。
func backoff(attempt int) time.Duration {
	d := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

func (d *Dispatcher) post(s *Subscriber, body []byte, sig, eventID string) (int, error) {
	client := d.client
	if client == nil {
		client = &http.Client{Timeout: s.timeout()}
	}
	req, err := http.NewRequest(http.MethodPost, s.URL, strings.NewReader(string(body)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DemoDog-Event", eventID)
	req.Header.Set("X-DemoDog-Signature", sig)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// Sign 返回 body 的小写十六进制 HMAC-SHA256，以 secret 作为 key，
// 格式为 "sha256=<hex>"。
func Sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Verify 校验由 Sign 生成的签名。
func Verify(body []byte, secret, signature string) bool {
	want := Sign(body, secret)
	return hmac.Equal([]byte(want), []byte(signature))
}

func (d *Dispatcher) recordDLQ(del Delivery) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.dlq) < d.dlqCap {
		d.dlq = append(d.dlq, del)
	} else {
		d.dlq[d.dlqHead] = del
		d.dlqHead = (d.dlqHead + 1) % d.dlqCap
	}
}

// DeadLetters 返回环形缓冲区的一个副本。
func (d *Dispatcher) DeadLetters() []Delivery {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Delivery, 0, len(d.dlq))
	if len(d.dlq) < d.dlqCap {
		out = append(out, d.dlq...)
	} else {
		for i := 0; i < len(d.dlq); i++ {
			idx := (d.dlqHead + i) % len(d.dlq)
			out = append(out, d.dlq[idx])
		}
	}
	return out
}

// Stats 是 JSON 稳定的视图。
type Stats struct {
	Subscribers int   `json:"subscribers"`
	Delivered   int64 `json:"delivered"`
	Failed      int64 `json:"failed"`
	DLQ         int   `json:"dlq"`
}

// Stats 返回当前 dispatcher 的计数器。
func (d *Dispatcher) Stats() Stats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return Stats{
		Subscribers: len(d.subscribers),
		Delivered:   d.countSent.Load(),
		Failed:      d.countFail.Load(),
		DLQ:         len(d.dlq),
	}
}
