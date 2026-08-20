package metrics

import (
	"bytes"
	"strings"
	"testing"
)

func TestRegistry_RegisterAndWrite(t *testing.T) {
	r := NewRegistry()
	c := NewCounterVec("req_total", "requests", []string{"tenant", "code"})
	r.MustRegister(c)
	s, _ := c.WithLabelValues("acme", "200")
	s.Inc()
	s.Inc()
	var buf bytes.Buffer
	r.WriteText(&buf)
	out := buf.String()
	if !strings.Contains(out, "# HELP req_total") {
		t.Fatal("missing HELP")
	}
	if !strings.Contains(out, "# TYPE req_total counter") {
		t.Fatal("missing TYPE")
	}
	if !strings.Contains(out, `req_total{tenant="acme",code="200"} 2`) {
		t.Fatalf("missing series: %s", out)
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	c := NewCounterVec("x", "", nil)
	r.MustRegister(c)
	if err := r.Register(c); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRegistry_BadName(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(NewCounterVec("1bad", "", nil)); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	c := NewCounterVec("x", "", nil)
	r.MustRegister(c)
	if _, ok := r.Get("x"); !ok {
		t.Fatal("missing")
	}
	if _, ok := r.Get("missing"); ok {
		t.Fatal("unexpected hit")
	}
}

func TestRegistry_Names(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(NewCounterVec("b", "", nil))
	r.MustRegister(NewCounterVec("a", "", nil))
	names := r.Names()
	if len(names) != 2 || names[0] != "a" {
		t.Fatalf("names: %v", names)
	}
}

func TestCounterVec_BadValues(t *testing.T) {
	c := NewCounterVec("x", "", []string{"a"})
	if _, err := c.WithLabelValues(); err == nil {
		t.Fatal("expected error")
	}
}

func TestCounterVec_Atomic(t *testing.T) {
	c := NewCounterVec("x", "", []string{"a"})
	s, _ := c.WithLabelValues("v")
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				s.Inc()
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if v := s.Value(); v != 10000 {
		t.Fatalf("value: %v", v)
	}
}

func TestGaugeVec(t *testing.T) {
	g := NewGaugeVec("g", "", []string{"a"})
	s, _ := g.WithLabelValues("v")
	s.Set(5)
	s.Inc()
	s.Dec()
	if s.Value() != 5 {
		t.Fatalf("value: %v", s.Value())
	}
}

func TestHistogramVec(t *testing.T) {
	h := NewHistogramVec("lat", "", []string{"path"}, []float64{1, 5, 10})
	s, _ := h.WithLabelValues("/a")
	s.Observe(0.5)
	s.Observe(3)
	s.Observe(20)
	var buf bytes.Buffer
	h.WriteText(&buf)
	out := buf.String()
	if !strings.Contains(out, `lat_bucket{path="/a",le="1"} 1`) {
		t.Fatalf("missing bucket 1: %s", out)
	}
	if !strings.Contains(out, `lat_bucket{path="/a",le="5"} 3`) {
		t.Fatalf("missing bucket 5: %s", out)
	}
	if !strings.Contains(out, `lat_bucket{path="/a",le="+Inf"} 3`) {
		t.Fatalf("missing +Inf bucket: %s", out)
	}
	if !strings.Contains(out, `lat_count{path="/a"} 3`) {
		t.Fatal("missing count")
	}
	if !strings.Contains(out, `lat_sum{path="/a"} 23.5`) {
		t.Fatal("missing sum")
	}
}

func TestHistogramVec_DefaultBuckets(t *testing.T) {
	h := NewHistogramVec("x", "", nil, nil)
	if len(h.buckets) != len(DefaultBuckets) {
		t.Fatal("default buckets")
	}
}

func TestWriteLabels(t *testing.T) {
	var buf bytes.Buffer
	WriteLabels(&buf, []string{"a", "b"}, []string{"1", "2"})
	if buf.String() != `{a="1",b="2"}` {
		t.Fatalf("got %s", buf.String())
	}
}

func TestWriteLabels_Escape(t *testing.T) {
	var buf bytes.Buffer
	WriteLabels(&buf, []string{"a"}, []string{`line1\nline2"quoted"`})
	if !strings.Contains(buf.String(), `\\n`) {
		t.Fatalf("missing escape: %s", buf.String())
	}
}

func TestValidName(t *testing.T) {
	if !validName("hello_world") {
		t.Fatal("valid")
	}
	if validName("1bad") {
		t.Fatal("invalid")
	}
	if validName("") {
		t.Fatal("empty")
	}
}

func TestEscapeHelp(t *testing.T) {
	if got := escapeHelp("a\\b\nc"); got != "a\\\\b\\nc" {
		t.Fatal(got)
	}
}
