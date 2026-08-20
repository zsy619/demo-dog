package routerx

import "testing"

func TestMultiParamAtSameLevel(t *testing.T) {
	r := New()
	r.Add("/users/:id", func(p map[string]string) {})
	r.Add("/users/:name", func(p map[string]string) {})
	h, p := r.Match("/users/abc")
	if h == nil {
		t.Fatal("no handler")
	}
	if _, ok := p["name"]; !ok {
		t.Fatalf("expected param name, got %v", p)
	}
}
