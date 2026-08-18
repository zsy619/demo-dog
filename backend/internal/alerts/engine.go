// Package alerts implements a tiny rules engine: load a list of
// alert rules + their SLO targets, evaluate them against the current
// engine state on every flush, and fire a webhook when an SLO burns
// through its error budget faster than allowed.
//
// The engine has zero external dependencies (stdlib only). Webhooks
// are POSTs of a small JSON envelope; receivers can fan out from
// there.
package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Rule struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Service     string        `json:"service,omitempty"`
	Target      float64       `json:"target"`
	Window      time.Duration `json:"window"`
	FastWindow  time.Duration `json:"fast_window"`
	FastBurn    float64       `json:"fast_burn"`
	SlowBurn    float64       `json:"slow_burn"`
	Severity    Severity      `json:"severity"`
	Channels    []string      `json:"channels"`
}

type Fire struct {
	Rule      Rule      `json:"rule"`
	Severity  Severity  `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
	Window    string    `json:"window"`
	Burn      float64   `json:"burn_rate"`
	Reason    string    `json:"reason"`
}

type Provider interface {
	SuccessRatio(service string, window time.Duration) (ratio float64, n int)
}

type Engine struct {
	mu       sync.Mutex
	rules    []Rule
	provider Provider
	client   *http.Client
	fires    []Fire
	firing   map[string]time.Time
	wg       sync.WaitGroup
}

func NewEngine(p Provider) *Engine {
	return &Engine{
		provider: p,
		client:   &http.Client{Timeout: 5 * time.Second},
		firing:   make(map[string]time.Time),
	}
}

func (e *Engine) SetRules(rules []Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append([]Rule(nil), rules...)
}

func (e *Engine) Recent(n int) []Fire {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n <= 0 || n > len(e.fires) {
		n = len(e.fires)
	}
	out := make([]Fire, n)
	copy(out, e.fires[len(e.fires)-n:])
	return out
}

func (e *Engine) Evaluate() {
	e.mu.Lock()
	rules := append([]Rule(nil), e.rules...)
	provider := e.provider
	client := e.client
	firing := make(map[string]time.Time, len(e.firing))
	for k, v := range e.firing {
		firing[k] = v
	}
	e.mu.Unlock()

	fires := []Fire{}
	for _, r := range rules {
		ratio, n := provider.SuccessRatio(r.Service, r.Window)
		if n == 0 {
			continue
		}
		errRate := 1 - ratio
		budget := 1 - r.Target
		if budget <= 0 {
			continue
		}
		burn := errRate / budget

		fastRatio, fastN := provider.SuccessRatio(r.Service, r.FastWindow)
		var fastBurn float64
		if fastN > 0 {
			fastBurn = (1 - fastRatio) / budget
		}

		var fired *Fire
		switch {
		case fastN > 0 && fastBurn >= r.FastBurn:
			f := Fire{
				Rule:      r,
				Severity:  r.Severity,
				Timestamp: time.Now().UTC(),
				Window:    "fast",
				Burn:      fastBurn,
				Reason:    fmt.Sprintf("burn rate %.2fx over %s (threshold %.2fx)", fastBurn, r.FastWindow, r.FastBurn),
			}
			fired = &f
		case burn >= r.SlowBurn:
			f := Fire{
				Rule:      r,
				Severity:  r.Severity,
				Timestamp: time.Now().UTC(),
				Window:    "slow",
				Burn:      burn,
				Reason:    fmt.Sprintf("burn rate %.2fx over %s (threshold %.2fx)", burn, r.Window, r.SlowBurn),
			}
			fired = &f
		}
		if fired == nil {
			continue
		}
		key := r.Name + "/" + fired.Window
		if last, ok := firing[key]; ok && time.Since(last) < 5*time.Minute {
			continue
		}
		firing[key] = time.Now()
		fires = append(fires, *fired)
		for _, ch := range r.Channels {
			e.wg.Add(1)
			go func(url string, f Fire) {
				defer e.wg.Done()
				e.postWebhook(url, f, client)
			}(ch, *fired)
		}
	}

	if len(fires) == 0 {
		return
	}
	e.mu.Lock()
	e.fires = append(e.fires, fires...)
	if len(e.fires) > 256 {
		e.fires = e.fires[len(e.fires)-256:]
	}
	e.firing = firing
	e.mu.Unlock()
}

func (e *Engine) postWebhook(url string, f Fire, client *http.Client) {
	body, _ := json.Marshal(f)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(io.Discard, "[alerts] webhook %s failed: %v\n", url, err)
		return
	}
	resp.Body.Close()
}

func (e *Engine) SortedRules() []Rule {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := append([]Rule(nil), e.rules...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
