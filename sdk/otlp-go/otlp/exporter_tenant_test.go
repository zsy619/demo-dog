package otlp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestExportTenantHeader confirms the exporter stamps X-Tenant-Id
// when WithTenantHeader is supplied. The backend reads this header
// before decoding the body so multi-tenant routing works without
// the caller having to embed tenant_id in the envelope.
func TestExportTenantHeader(t *testing.T) {
	var gotTenant string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-Id")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(Response{})
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL, WithTimeout(0), WithTenantHeader("acme"))
	if _, err := exp.Export(context.Background(), Request{ResourceAttrs: map[string]string{"service.name": "x"}}); err != nil {
		t.Fatal(err)
	}
	if gotTenant != "acme" {
		t.Fatalf("X-Tenant-Id: %q", gotTenant)
	}
}

// TestExportNoTenantHeader confirms we do not emit X-Tenant-Id when
// WithTenantHeader is omitted (the header would be the empty string
// and the backend would treat it as anonymous).
func TestExportNoTenantHeader(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Tenant-Id")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(Response{})
	}))
	defer srv.Close()

	exp := NewExporter(srv.URL, WithTimeout(0))
	if _, err := exp.Export(context.Background(), Request{ResourceAttrs: map[string]string{"service.name": "x"}}); err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("X-Tenant-Id should be empty, got %q", got)
	}
}
