package openapi

import (
	"encoding/json"
	"testing"
)

func TestSpec_ValidJSON(t *testing.T) {
	s := New()
	data, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip Spec
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if roundtrip.OpenAPI != "3.1.0" {
		t.Fatalf("OpenAPI: %s", roundtrip.OpenAPI)
	}
}

func TestSpec_HasAllPaths(t *testing.T) {
	s := New()
	want := []string{
		"/api/health",
		"/api/v1/query",
		"/api/v1/series",
		"/api/v1/rules",
		"/v1/logs",
		"/v1/metrics",
		"/v1/traces",
		"/api/v1/write",
		"/api/tenants",
		"/api/keys",
		"/api/snapshot",
		"/replica/ack",
		"/replica/state",
		"/replica/wal",
	}
	for _, path := range want {
		if _, ok := s.Paths[path]; !ok {
			t.Errorf("missing path: %s", path)
		}
	}
}

func TestSpec_HasSecuritySchemes(t *testing.T) {
	s := New()
	if _, ok := s.Components.SecuritySchemes["ApiKey"]; !ok {
		t.Fatal("missing ApiKey security scheme")
	}
	if _, ok := s.Components.SecuritySchemes["Bearer"]; !ok {
		t.Fatal("missing Bearer security scheme")
	}
}

func TestSpec_HasInfo(t *testing.T) {
	s := New()
	if s.Info.Title == "" {
		t.Fatal("title missing")
	}
	if s.Info.Version == "" {
		t.Fatal("version missing")
	}
}

func TestSpec_PathHasMethod(t *testing.T) {
	s := New()
	if s.Paths["/api/health"].Get == nil {
		t.Fatal("/api/health should have GET")
	}
	if s.Paths["/v1/logs"].Post == nil {
		t.Fatal("/v1/logs should have POST")
	}
	if s.Paths["/replica/ack"].Post == nil {
		t.Fatal("/replica/ack should have POST")
	}
}

func TestSpec_Responses(t *testing.T) {
	s := New()
	for path, item := range s.Paths {
		if item.Get != nil && len(item.Get.Responses) == 0 {
			t.Errorf("GET %s has no responses", path)
		}
		if item.Post != nil && len(item.Post.Responses) == 0 {
			t.Errorf("POST %s has no responses", path)
		}
	}
}

func TestSpec_SchemasReachable(t *testing.T) {
	s := New()
	if _, ok := s.Components.Schemas["Health"]; !ok {
		t.Fatal("Health schema missing")
	}
	if _, ok := s.Components.Schemas["QueryResponse"]; !ok {
		t.Fatal("QueryResponse schema missing")
	}
	if _, ok := s.Components.Schemas["RulesGroup"]; !ok {
		t.Fatal("RulesGroup schema missing")
	}
}
