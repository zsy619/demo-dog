package otlp

import (
	"context"
	"testing"
)

func TestWithAutoResource(t *testing.T) {
	srv := startAcceptedServer(t)
	defer srv.Close()
	sdk, err := New(srv.URL,
		WithService("semconv-test"),
		WithAutoResource(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer sdk.Shutdown(context.Background())

	sdk.Log(context.Background(), SeverityInfo, "x")
	if err := sdk.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}

	resource := sdk.Snapshot().ResourceAttrs
	want := []string{"process.pid", "process.runtime.name", "host.arch", "os.type"}
	for _, k := range want {
		if _, ok := resource[k]; !ok {
			t.Errorf("missing %q in resource: %+v", k, resource)
		}
	}
}

func TestWithoutAutoResource(t *testing.T) {
	srv := startAcceptedServer(t)
	defer srv.Close()
	sdk, _ := New(srv.URL, WithService("semconv-test"))
	defer sdk.Shutdown(context.Background())

	sdk.Log(context.Background(), SeverityInfo, "x")
	_ = sdk.ForceFlush(context.Background())

	resource := sdk.Snapshot().ResourceAttrs
	if _, ok := resource["process.pid"]; ok {
		t.Errorf("process.pid should not be present by default: %+v", resource)
	}
}
