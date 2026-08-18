// HTTP exporter. Sends an otlp.Request to the DOG collector ingest
// endpoint and parses the OTLPResponse.
//
// Wire compatibility:
//   - POST {endpoint}/api/ingest/otlp-json
//   - Content-Type: application/json+otlp
//   - Body: JSON encoding of otlp.Request
//
// The exporter uses net/http directly so the SDK has no external
// dependencies. Callers who need auth headers, custom TLS, or a proxy can
// configure the underlying *http.Client via WithHTTPClient.
package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Exporter sends an otlp.Request to the collector ingest endpoint.
type Exporter struct {
	endpoint   string
	httpClient *http.Client

	// apiKey, when non-empty, is attached to every outgoing request
	// as `Authorization: Bearer <key>`. The collector accepts the
	// same value via X-API-Key or ?api_key=... for debug use, but
	// the bearer header is the recommended path for production.
	apiKey string
}

// ExporterOption mutates the Exporter during construction.
type ExporterOption func(*Exporter)

// WithHTTPClient lets callers bring their own *http.Client (auth
// roundtrippers, custom TLS, proxies).
func WithHTTPClient(c *http.Client) ExporterOption {
	return func(e *Exporter) {
		if c != nil {
			e.httpClient = c
		}
	}
}

// WithEndpoint overrides the ingest endpoint. The default is
// {base}/api/ingest/otlp-json. Pass a fully qualified URL if your
// collector mounts ingest elsewhere.
func WithEndpoint(url string) ExporterOption {
	return func(e *Exporter) {
		e.endpoint = url
	}
}

// WithTimeout overrides the per-request timeout. Default is 10s.
func WithTimeout(d time.Duration) ExporterOption {
	return func(e *Exporter) {
		if e.httpClient != nil {
			e.httpClient.Timeout = d
		}
	}
}

// WithAPIKey attaches a static bearer token to every outgoing request.
// The collector must have the same key registered via -api-keys or
// DOG_API_KEYS; otherwise the request is rejected with 401.
//
// Empty key is a no-op (useful for tests / dev mode).
func WithAPIKey(key string) ExporterOption {
	return func(e *Exporter) {
		e.apiKey = key
	}
}

// NewExporter builds a default Exporter. By default the SDK talks to the
// simplified JSON ingest endpoint (/api/ingest/otlp) using the
// otlp.Request wire schema. Set WithEndpoint to switch to the OTel JSON
// envelope (/api/ingest/otlp-json) if you want to talk to a vanilla OTel
// collector that expects the spec-format envelope.
func NewExporter(base string, opts ...ExporterOption) *Exporter {
	e := &Exporter{
		endpoint:   joinEndpoint(base, "/api/ingest/otlp"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Export POSTs the request and decodes the response.
func (e *Exporter) Export(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if e.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("transport: %w", err)
	}
	defer resp.Body.Close()

	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ingest rejected: status=%d body=%s", resp.StatusCode, string(rb))
	}

	var out Response
	if err := json.Unmarshal(rb, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w (body=%s)", err, string(rb))
	}
	return &out, nil
}

// joinEndpoint trims slashes between base and path.
func joinEndpoint(base, path string) string {
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	if base == "" {
		return "/" + path
	}
	return base + "/" + path
}
