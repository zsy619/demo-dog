package alerts

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProvider struct {
	ratio float64
	n     int
}

func (f *fakeProvider) SuccessRatio(string, time.Duration) (float64, int) {
	return f.ratio, f.n
}

func TestEngine_FiresOnFastBurn(t *testing.T) {
	// Success ratio 0.5 over a 30m window with SLO target 0.99 ->
	// burn = 0.5 / 0.01 = 50x. Should fire immediately.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()

	e := NewEngine(&fakeProvider{ratio: 0.5, n: 100})
	r := Rule{
		Name:       "checkout-availability",
		Service:    "checkout",
		Target:     0.99,
		Window:     30 * time.Minute,
		FastWindow: 5 * time.Minute,
		FastBurn:   2,
		SlowBurn:   1,
		Severity:   SeverityCritical,
		Channels:   []string{srv.URL},
	}
	e.SetRules([]Rule{r})
	e.Evaluate()

	if got := e.Recent(10); len(got) == 0 {
		t.Errorf("expected at least one fire")
	}
	e.wg.Wait()
	if atomic.LoadInt32(&hits) == 0 {
		t.Errorf("webhook was not called")
	}
}

func TestEngine_NoFireWhenBudget(t *testing.T) {
	// Success ratio 0.999, target 0.99 -> burn = 0.001 / 0.01 = 0.1x.
	e := NewEngine(&fakeProvider{ratio: 0.999, n: 100})
	r := Rule{
		Name:       "checkout-availability",
		Target:     0.99,
		Window:     30 * time.Minute,
		FastWindow: 5 * time.Minute,
		FastBurn:   2,
		SlowBurn:   1,
		Severity:   SeverityWarning,
	}
	e.SetRules([]Rule{r})
	e.Evaluate()
	if got := e.Recent(10); len(got) != 0 {
		t.Errorf("expected zero fires, got %d", len(got))
	}
}

func TestEngine_SkipsEmptyWindow(t *testing.T) {
	e := NewEngine(&fakeProvider{ratio: 0, n: 0}) // no data
	r := Rule{Name: "x", Target: 0.99, Window: time.Minute, FastWindow: time.Second, FastBurn: 1, SlowBurn: 1}
	e.SetRules([]Rule{r})
	e.Evaluate()
	if got := e.Recent(10); len(got) != 0 {
		t.Errorf("expected zero fires for empty window")
	}
}

func TestEngine_DedupePerRule(t *testing.T) {
	e := NewEngine(&fakeProvider{ratio: 0.5, n: 100})
	r := Rule{
		Name: "x", Target: 0.99,
		Window: 30 * time.Minute, FastWindow: 5 * time.Minute,
		FastBurn: 1, SlowBurn: 1, Severity: SeverityCritical,
	}
	e.SetRules([]Rule{r})
	e.Evaluate()
	e.Evaluate() // second call within 5 min should be deduped
	if got := e.Recent(10); len(got) != 1 {
		t.Errorf("expected 1 fire after dedupe, got %d", len(got))
	}
}
