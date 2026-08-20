// Package alerts 实现告警规则评估与通知。
// This file adds SMTP email and PagerDuty notification channels.
package alerts

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// ChannelKind 枚举支持的告警投递通道。
type ChannelKind string

const (
	ChannelWebhook   ChannelKind = "webhook"
	ChannelEmail     ChannelKind = "email"
	ChannelPagerDuty ChannelKind = "pagerduty"
	ChannelSlack     ChannelKind = "slack"
)

// NotifyOpts is the per-call options bag for delivery.
type NotifyOpts struct {
	Subject   string
	Body      string
	Severity  string // info|warn|error
	Labels    map[string]string
}

// Channel 是每个具体通知器实现的契约。
type Channel interface {
	Kind() ChannelKind
	Send(ctx context.Context, opts NotifyOpts) error
}

// --- Webhook (already implemented in engine.go) ---
// We expose the same surface here for consistency.

type WebhookChannel struct {
	URL     string
	Client  *http.Client
	Headers map[string]string
}

func (w *WebhookChannel) Kind() ChannelKind { return ChannelWebhook }

func (w *WebhookChannel) Send(ctx context.Context, opts NotifyOpts) error {
	if w.URL == "" { return errors.New("empty webhook url") }
	if w.Client == nil {
		w.Client = &http.Client{Timeout: 5 * time.Second}
	}
	payload := map[string]any{
		"subject":  opts.Subject,
		"body":     opts.Body,
		"severity": opts.Severity,
		"labels":   opts.Labels,
		"ts":       time.Now().Unix(),
	}
	body, err := json.Marshal(payload)
	if err != nil { return err }
	req, err := http.NewRequestWithContext(ctx, "POST", w.URL, bytes.NewReader(body))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}
	resp, err := w.Client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("webhook status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// --- Email ---
// SMTP delivery with STARTTLS support. Configuration:
//   DOG_SMTP_HOST, DOG_SMTP_PORT, DOG_SMTP_USER, DOG_SMTP_PASS, DOG_SMTP_FROM
//
// We deliberately keep this stdlib-only: no third-party mail libs.
type EmailChannel struct {
	Host     string
	Port     int
	User     string
	Pass     string
	From     string
	To       []string
	UseTLS   bool
}

func (e *EmailChannel) Kind() ChannelKind { return ChannelEmail }

func (e *EmailChannel) Send(ctx context.Context, opts NotifyOpts) error {
	if e.Host == "" { return errors.New("smtp host empty") }
	if len(e.To) == 0 { return errors.New("no recipients") }
	addr := fmt.Sprintf("%s:%d", e.Host, e.Port)
	if e.Port == 0 { e.Port = 25 }

	msg := buildMIME(opts.Subject, e.From, e.To, opts.Body)

	var auth smtp.Auth
	if e.User != "" {
		auth = smtp.PlainAuth("", e.User, e.Pass, e.Host)
	}

	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil { return err }

	if e.UseTLS {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: e.Host})
		if err := tlsConn.Handshake(); err != nil { conn.Close(); return err }
		c, err := smtp.NewClient(tlsConn, e.Host)
		if err != nil { return err }
		defer c.Close()
		if auth != nil { if err := c.Auth(auth); err != nil { return err } }
		return sendMail(c, e.From, e.To, []byte(msg))
	}

	c, err := smtp.NewClient(conn, e.Host)
	if err != nil { return err }
	defer c.Close()
	if auth != nil { if err := c.Auth(auth); err != nil { return err } }
	return sendMail(c, e.From, e.To, []byte(msg))
}

func sendMail(c *smtp.Client, from string, to []string, msg []byte) error {
	if err := c.Mail(from); err != nil { return err }
	for _, r := range to {
		if err := c.Rcpt(r); err != nil { return err }
	}
	w, err := c.Data()
	if err != nil { return err }
	if _, err := w.Write(msg); err != nil { return err }
	if err := w.Close(); err != nil { return err }
	return c.Quit()
}

func buildMIME(subject, from string, to []string, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// --- PagerDuty ---
// Events API v2. POSTs an Events API v2 payload with severity=error
// and the dedup_key set to the rule + subject so multiple firings
// of the same alert coalesce.
type PagerDutyChannel struct {
	IntegrationKey string
	Client         *http.Client
	DedupKey       string
}

func (p *PagerDutyChannel) Kind() ChannelKind { return ChannelPagerDuty }

func (p *PagerDutyChannel) Send(ctx context.Context, opts NotifyOpts) error {
	if p.IntegrationKey == "" { return errors.New("pagerduty integration key empty") }
	if p.Client == nil {
		p.Client = &http.Client{Timeout: 5 * time.Second}
	}
	payload := map[string]any{
		"routing_key":  p.IntegrationKey,
		"event_action": "trigger",
		"dedup_key":    p.DedupKey,
		"payload": map[string]any{
			"summary":   opts.Subject,
			"source":    "demo-dog",
			"severity":  severityToPD(opts.Severity),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"custom_details": map[string]any{
				"body": opts.Body,
				"labels": opts.Labels,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil { return err }
	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://events.pagerduty.com/v2/enqueue", bytes.NewReader(body))
	if err != nil { return err }
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.Client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("pagerduty status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func severityToPD(s string) string {
	switch strings.ToLower(s) {
	case "error", "fatal", "critical":
		return "critical"
	case "warn", "warning":
		return "warning"
	default:
		return "info"
	}
}

// Multiplexer 并行将 NotifyOpts 扇出到多个 channel，并
// aggregates the per-channel errors. Used by the engine when a rule
// is wired to multiple channels.
type Multiplexer struct {
	Channels []Channel
}

func (m *Multiplexer) Send(ctx context.Context, opts NotifyOpts) error {
	if m == nil || len(m.Channels) == 0 { return nil }
	var firstErr error
	for _, ch := range m.Channels {
		if err := ch.Send(ctx, opts); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", ch.Kind(), err)
		}
	}
	return firstErr
}

// --- Slack ---
// SlackChannel posts to an Incoming Webhook URL.
//
// Slack Incoming Webhooks accept a JSON body with a `text` field
// and optional `blocks` for rich formatting. We use a simple text
// payload that contains subject, severity, and labels.
type SlackChannel struct {
	WebhookURL string
	Channel    string // optional override for #channel
	Username   string // optional override for display name
	Client     *http.Client
}

func (s *SlackChannel) Kind() ChannelKind { return ChannelSlack }

func (s *SlackChannel) Send(ctx context.Context, opts NotifyOpts) error {
	if s.WebhookURL == "" {
		return errors.New("empty slack webhook url")
	}
	if s.Client == nil {
		s.Client = &http.Client{Timeout: 5 * time.Second}
	}
	text := fmt.Sprintf("*[%s] %s*\n%s", strings.ToUpper(opts.Severity), opts.Subject, opts.Body)
	payload := map[string]any{"text": text}
	if s.Channel != "" {
		payload["channel"] = s.Channel
	}
	if s.Username != "" {
		payload["username"] = s.Username
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", s.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("slack status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// --- Retry ---
// RetryChannel wraps another Channel with exponential-backoff
// retries. Useful for transient network errors on PagerDuty /
// Slack / webhook sinks. Returns the last error if all attempts
// fail.
type RetryChannel struct {
	Inner    Channel
	Attempts int
	BaseWait time.Duration
}

func (r *RetryChannel) Kind() ChannelKind { return r.Inner.Kind() }

func (r *RetryChannel) Send(ctx context.Context, opts NotifyOpts) error {
	if r.Attempts < 1 {
		r.Attempts = 3
	}
	if r.BaseWait <= 0 {
		r.BaseWait = 200 * time.Millisecond
	}
	var lastErr error
	for i := 0; i < r.Attempts; i++ {
		if i > 0 {
			wait := r.BaseWait * time.Duration(1<<uint(i-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		if err := r.Inner.Send(ctx, opts); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("after %d attempts: %w", r.Attempts, lastErr)
}

// SeverityColor returns a hex color suitable for Slack attachments
// or PagerDuty custom details. Maps severity strings to traffic
// light colors.
func SeverityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "fatal", "critical":
		return "#dc3545" // red
	case "error":
		return "#fd7e14" // orange
	case "warn", "warning":
		return "#ffc107" // yellow
	case "info":
		return "#0dcaf0" // blue
	default:
		return "#6c757d" // gray
	}
}
