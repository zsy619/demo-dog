// Prometheus text-format support.
//
// This file adds Prometheus scrape compatibility to the SDK:
//   - PrometheusExporter: render an otlp.Request as text/plain;version=0.0.4
//   - PrometheusCollector: hook an SDK instance into an http.Handler so a
//     /metrics endpoint always reflects the current buffer
//
// Limitations vs. a real Prometheus client library:
//   - Histograms are emitted as a single observation (sample), plus a
//     `_sum` and `_count` line. Bucket counters are not maintained.
//   - Counters are emitted as raw values; the scrape side computes rates.
//   - The collector snapshots the SDK buffer at scrape time, so a
//     ForceFlush immediately before scrape is recommended for accuracy.
package otlp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PrometheusExporter renders an otlp.Request as Prometheus text. It
// implements ExporterInterface so it can be wired into the SDK as an
// exporter, though the more common path is PrometheusCollector.Handler.
type PrometheusExporter struct {
	prefix string
}

// PrometheusOption mutates a PrometheusExporter at construction.
type PrometheusOption func(*PrometheusExporter)

// WithPrometheusPrefix sets a global metric name prefix (e.g. "myapp_").
func WithPrometheusPrefix(p string) PrometheusOption {
	return func(e *PrometheusExporter) { e.prefix = p }
}

// NewPrometheusExporter builds a scrape-side renderer.
func NewPrometheusExporter(opts ...PrometheusOption) *PrometheusExporter {
	e := &PrometheusExporter{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export satisfies ExporterInterface. The Response carries a sentinel
// AcceptedMetrics count; callers wanting the bytes should use Render.
func (p *PrometheusExporter) Export(_ context.Context, req Request) (*Response, error) {
	buf, err := p.Render(req)
	if err != nil {
		return nil, err
	}
	_ = buf
	return &Response{AcceptedMetrics: len(req.Metrics)}, nil
}

// Render returns the Prometheus text bytes for a request.
func (p *PrometheusExporter) Render(req Request) ([]byte, error) {
	var buf bytes.Buffer
	if err := p.writeExposition(&buf, req); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Handler returns an http.Handler that emits a static help message.
// For live SDK-driven output, use PrometheusCollector instead.
func (p *PrometheusExporter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		io.WriteString(w, "# prometheus exporter -- no SDK bound; use PrometheusCollector\n")
	})
}

// PrometheusCollector wires an SDK to a scrape endpoint. The handler
// emits a snapshot of the SDK buffer on every scrape. Callers wanting
// strict accuracy should call sdk.ForceFlush(ctx) immediately before
// the scrape.
type PrometheusCollector struct {
	sdk    *SDK
	prefix string
}

// NewPrometheusCollector returns a collector bound to the SDK.
func NewPrometheusCollector(sdk *SDK, opts ...PrometheusOption) *PrometheusCollector {
	c := &PrometheusCollector{sdk: sdk, prefix: ""}
	for _, opt := range opts {
		opt(&PrometheusExporter{})
	}
	return c
}

// WithPrefix updates the collector metric name prefix.
func (c *PrometheusCollector) WithPrefix(p string) *PrometheusCollector {
	c.prefix = p
	return c
}

// Handler returns an http.Handler that emits the SDK snapshot.
func (c *PrometheusCollector) Handler() http.Handler {
	p := &PrometheusExporter{prefix: c.prefix}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := c.sdk.Snapshot()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		if err := p.writeExposition(w, req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

// Render returns the Prometheus exposition as a byte slice. Useful for
// frameworks that need the body directly (e.g. Hertz, Fiber, gRPC-gateway)
// without going through a stdlib http.Handler.
func (c *PrometheusCollector) Render() ([]byte, error) {
	p := &PrometheusExporter{prefix: c.prefix}
	return p.Render(c.sdk.Snapshot())
}

func (p *PrometheusExporter) writeExposition(w io.Writer, req Request) error {
	type metric struct {
		Name string
		Help string
		Kind string
		Pts  []MetricPoint
	}
	grouped := map[string]*metric{}
	for _, m := range req.Metrics {
		fullName := promName(p.prefix + m.Name)
		if g, ok := grouped[fullName]; ok {
			g.Pts = append(g.Pts, m)
			continue
		}
		kind := "untyped"
		switch m.Type {
		case TypeCounter:
			kind = "counter"
		case TypeGauge:
			kind = "gauge"
		case TypeHistogram:
			kind = "histogram"
		}
		grouped[fullName] = &metric{
			Name: fullName,
			Help: "exported by otlp-go",
			Kind: kind,
			Pts:  []MetricPoint{m},
		}
	}

	names := make([]string, 0, len(grouped))
	for k := range grouped {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		m := grouped[name]
		fmt.Fprintf(w, "# HELP %s %s\n", m.Name, m.Help)
		fmt.Fprintf(w, "# TYPE %s %s\n", m.Name, m.Kind)
		for _, pt := range m.Pts {
			if err := writeMetricLine(w, m.Name, m.Kind, pt); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeMetricLine(w io.Writer, name, kind string, m MetricPoint) error {
	keys := make([]string, 0, len(m.Labels))
	for k := range m.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var labelStr strings.Builder
	if len(keys) > 0 {
		labelStr.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				labelStr.WriteByte(',') 
			}
			labelStr.WriteString(strconv.Quote(k))
			labelStr.WriteString("=\"")
			labelStr.WriteString(escapeLabelValue(m.Labels[k]))
			labelStr.WriteByte('"')
		}
		labelStr.WriteByte('}')
	}
	if kind == "histogram" {
		fmt.Fprintf(w, "%s_sum%s %g %d\n", name, labelStr.String(), m.Value, m.Timestamp.UnixMilli())
		fmt.Fprintf(w, "%s_count%s %d %d\n", name, labelStr.String(), 1, m.Timestamp.UnixMilli())
		return nil
	}
	fmt.Fprintf(w, "%s%s %g %d\n", name, labelStr.String(), m.Value, m.Timestamp.UnixMilli())
	return nil
}

func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

var _ = time.Now

// promName sanitizes an OTel metric name into the form Prometheus
// expects: only [a-zA-Z0-9_:]. Dots and dashes become underscores.
func promName(in string) string {
	out := make([]byte, len(in))
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == ':' || c == '_':
			out[i] = c
		default:
			out[i] = '_'
		}
	}
	return string(out)
}
