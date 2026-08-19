package metricx

import (
	"bytes"
	"strings"
	"testing"
)

func TestCounter(t *testing.T) {
	p := NewPusher()
	c := p.Counter("hits")
	c.Inc()
	c.Add(5)
	if c.Value() != 6 {
		t.Fatal("val")
	}
}

func TestWriteSnapshot(t *testing.T) {
	p := NewPusher()
	p.Counter("a").Add(1)
	p.Counter("b").Add(2)
	var buf bytes.Buffer
	if err := p.WriteSnapshot(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "a 1") || !strings.Contains(out, "b 2") {
		t.Fatal("snapshot")
	}
}

func TestNames(t *testing.T) {
	p := NewPusher()
	p.Counter("a")
	p.Counter("b")
	if len(p.Names()) != 2 {
		t.Fatal("names")
	}
}

func TestReset(t *testing.T) {
	p := NewPusher()
	p.Counter("a")
	p.Reset()
	if len(p.Names()) != 0 {
		t.Fatal("reset")
	}
}
