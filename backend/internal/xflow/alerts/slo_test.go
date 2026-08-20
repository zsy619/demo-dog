package alerts

import (
	"math"
	"testing"
	"time"
)

type mapSink map[string]int64

func (m mapSink) Counter(name string, _ time.Duration) int64 {
	return m[name]
}

func TestSLO_Validate(t *testing.T) {
	cases := []struct {
		name string
		s    SLO
		err  bool
	}{
		{"valid", SLO{Name: "x", Target: 0.99, Window: time.Hour}, false},
		{"no name", SLO{Target: 0.99, Window: time.Hour}, true},
		{"target=0", SLO{Name: "x", Window: time.Hour}, true},
		{"target=1", SLO{Name: "x", Target: 1.0, Window: time.Hour}, true},
		{"no window", SLO{Name: "x", Target: 0.99}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.s.Validate()
			if (err != nil) != c.err {
				t.Fatalf("err=%v want %v", err, c.err)
			}
		})
	}
}

func TestSLO_Budget(t *testing.T) {
	s := SLO{Target: 0.99}
	if math.Abs(s.Budget()-0.01) > 1e-9 {
		t.Fatalf("budget: %v", s.Budget())
	}
}

func TestCompute_AllGood(t *testing.T) {
	sink := mapSink{"req_total": 1000, "req_bad": 0}
	s := SLO{Name: "availability", Target: 0.99, Window: time.Hour, TotalCounter: "req_total", BadCounter: "req_bad"}
	b, err := Compute(&s, sink, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !b.Healthy {
		t.Fatal("expected healthy")
	}
	if b.BudgetLeftPercent != 100 {
		t.Fatalf("budget left: %f", b.BudgetLeftPercent)
	}
}

func TestCompute_BurntOut(t *testing.T) {
	sink := mapSink{"req_total": 100, "req_bad": 10}
	s := SLO{Name: "avail", Target: 0.99, Window: time.Hour, TotalCounter: "req_total", BadCounter: "req_bad"}
	b, err := Compute(&s, sink, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if b.Healthy {
		t.Fatal("expected unhealthy (10% > 1%)")
	}
	if b.BudgetLeftPercent != 0 {
		t.Fatalf("expected 0, got %f", b.BudgetLeftPercent)
	}
}

func TestCompute_HalfBurnt(t *testing.T) {
	sink := mapSink{"req_total": 1000, "req_bad": 5}
	s := SLO{Name: "avail", Target: 0.99, Window: time.Hour, TotalCounter: "req_total", BadCounter: "req_bad"}
	b, _ := Compute(&s, sink, time.Now())
	if !b.Healthy {
		t.Fatal("5/1000 = 0.5% < 1% budget")
	}
	if b.BudgetLeftPercent < 49 || b.BudgetLeftPercent > 51 {
		t.Fatalf("expected ~50%%, got %f", b.BudgetLeftPercent)
	}
}

func TestCompute_BadGreaterThanTotal(t *testing.T) {
	sink := mapSink{"req_total": 10, "req_bad": 50}
	s := SLO{Name: "avail", Target: 0.99, Window: time.Hour, TotalCounter: "req_total", BadCounter: "req_bad"}
	b, _ := Compute(&s, sink, time.Now())
	if b.Bad != b.Total {
		t.Fatalf("bad should be clamped to total")
	}
}

func TestBurnRates(t *testing.T) {
	sink := mapSink{"req_total": 100, "req_bad": 2}
	s := SLO{Name: "avail", Target: 0.99, Window: time.Hour, TotalCounter: "req_total", BadCounter: "req_bad"}
	burns, err := BurnRates(&s, sink)
	if err != nil {
		t.Fatal(err)
	}
	if len(burns) != 8 {
		t.Fatalf("expected 8 windows, got %d", len(burns))
	}
	// All windows have the same data here (sink is key-only); 2/100 = 2% rate, 0.01 budget -> 2x burn.
	for _, b := range burns {
		if math.Abs(b.Rate-2) > 0.01 {
			t.Errorf("burn at %v: %f", b.Window, b.Rate)
		}
	}
}

func TestDecide_None(t *testing.T) {
	d := Decide(BurnRate{Window: 5 * time.Minute, Rate: 0.5}, BurnRate{Window: time.Hour, Rate: 0.5})
	if d.Level != "none" {
		t.Fatalf("expected none, got %s", d.Level)
	}
}

func TestDecide_PageFast(t *testing.T) {
	d := Decide(BurnRate{Window: 5 * time.Minute, Rate: 14.4}, BurnRate{Window: time.Hour, Rate: 14.4})
	if d.Level != "page" {
		t.Fatalf("expected page, got %s (%s)", d.Level, d.Reason)
	}
}

func TestDecide_PageSlow(t *testing.T) {
	d := Decide(BurnRate{Window: 30 * time.Minute, Rate: 6}, BurnRate{Window: 6 * time.Hour, Rate: 6})
	if d.Level != "page" {
		t.Fatalf("expected page, got %s (%s)", d.Level, d.Reason)
	}
}

func TestDecide_WarnTicket(t *testing.T) {
	d := Decide(BurnRate{Window: 3 * 24 * time.Hour, Rate: 3}, BurnRate{Window: 6 * time.Hour, Rate: 3})
	if d.Level != "warn" {
		t.Fatalf("expected warn, got %s (%s)", d.Level, d.Reason)
	}
}

func TestDecide_WarnElevated(t *testing.T) {
	d := Decide(BurnRate{Window: 5 * time.Minute, Rate: 3}, BurnRate{Window: time.Hour, Rate: 3})
	if d.Level != "warn" {
		t.Fatalf("expected warn, got %s (%s)", d.Level, d.Reason)
	}
}

func TestScore_Healthy(t *testing.T) {
	s := BudgetStatus{Budget: 0.01, BudgetLeft: 0.01}
	if Score(s) < 0.99 {
		t.Fatalf("score: %v", Score(s))
	}
}

func TestScore_Exhausted(t *testing.T) {
	s := BudgetStatus{Budget: 0.01, BudgetLeft: 0}
	if Score(s) > 0.05 {
		t.Fatalf("score should be near 0: %v", Score(s))
	}
}

func TestScore_Half(t *testing.T) {
	s := BudgetStatus{Budget: 0.01, BudgetLeft: 0.005}
	v := Score(s)
	if v < 0.4 || v > 0.6 {
		t.Fatalf("expected ~0.5, got %v", v)
	}
}

func TestScore_NoBudget(t *testing.T) {
	s := BudgetStatus{Budget: 0}
	if Score(s) != 1 {
		t.Fatalf("no budget should score 1, got %v", Score(s))
	}
}
