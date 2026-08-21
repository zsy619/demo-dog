package alerts

// evaluate.go:Evaluate + webhook 投递。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Evaluate 评估所有规则,必要时触发 Fire + webhook。
//
// 每次 Evaluate 调用视为一次原子快照——使用锁外副本以避免长时间持锁。
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

// postWebhook 把一条 Fire 序列化为 JSON POST 到 webhook URL。
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
