package replica

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuth_Disabled(t *testing.T) {
	a := NewAuth(nil)
	if a.Enabled() {
		t.Fatal("empty auth should be disabled")
	}
	r := httptest.NewRequest("GET", "/", nil)
	if id := a.Authenticate(r); id != "anon" {
		t.Fatalf("anon: %s", id)
	}
}

func TestAuth_Valid(t *testing.T) {
	a := NewAuth([]string{"s3cret:follower-a", "other:follower-b"})
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer s3cret")
	if id := a.Authenticate(r); id != "follower-a" {
		t.Fatalf("got: %s", id)
	}
}

func TestAuth_Invalid(t *testing.T) {
	a := NewAuth([]string{"s3cret:follower-a"})
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer wrong")
	if id := a.Authenticate(r); id != "" {
		t.Fatalf("expected empty, got: %s", id)
	}
}

func TestAuth_MissingHeader(t *testing.T) {
	a := NewAuth([]string{"s3cret:follower-a"})
	r := httptest.NewRequest("GET", "/", nil)
	if id := a.Authenticate(r); id != "" {
		t.Fatalf("expected empty, got: %s", id)
	}
}

func TestAuth_WrongScheme(t *testing.T) {
	a := NewAuth([]string{"s3cret:follower-a"})
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	if id := a.Authenticate(r); id != "" {
		t.Fatalf("expected empty for basic auth, got: %s", id)
	}
}

func TestAuth_Middleware_Blocks(t *testing.T) {
	a := NewAuth([]string{"s3cret:follower-a"})
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_Middleware_Passes(t *testing.T) {
	a := NewAuth([]string{"s3cret:follower-a"})
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL, nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuth_SkipsMalformedEntry(t *testing.T) {
	a := NewAuth([]string{"no_colon_here", ":empty_token", "good:id"})
	if !a.Enabled() {
		t.Fatal("should be enabled with one good entry")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer good")
	if id := a.Authenticate(r); id != "id" {
		t.Fatalf("got: %s", id)
	}
}

func TestTLSConfigFromPairs_BadInput(t *testing.T) {
	_, err := TLSConfigFromPairs([]byte("not pem"), []byte("not pem"))
	if err == nil {
		t.Fatal("expected error for non-PEM input")
	}
}
