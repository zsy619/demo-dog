package oauth

import (
	"testing"
	"time"
)

func newServer() *Server {
	return New("https://example.com", "secret-key").WithTime(func() time.Time { return time.Unix(1700000000, 0) })
}

func TestRegisterAndLookup(t *testing.T) {
	s := newServer()
	s.Register(&Client{ID: "c1", Secret: []byte("s1"), Scopes: []string{"read"}, Tenant: "t1"})
	c, ok := s.ClientLookup("c1")
	if !ok || c.Tenant != "t1" {
		t.Fatal("lookup")
	}
	if _, ok := s.ClientLookup("missing"); ok {
		t.Fatal("missing")
	}
}

func TestIssue_Success(t *testing.T) {
	s := newServer()
	s.Register(&Client{ID: "c1", Secret: []byte("s1"), Scopes: []string{"read", "write"}, Tenant: "t1"})
	r, err := s.IssueClientCredentials("c1", "s1", []string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	if r.TokenType != "Bearer" {
		t.Fatal("token type")
	}
	if r.ExpiresIn <= 0 {
		t.Fatal("expires_in")
	}
	if r.AccessToken == "" || r.RefreshToken == "" {
		t.Fatal("missing token")
	}
}

func TestIssue_BadSecret(t *testing.T) {
	s := newServer()
	s.Register(&Client{ID: "c1", Secret: []byte("s1")})
	if _, err := s.IssueClientCredentials("c1", "wrong", nil); err != ErrInvalidClient {
		t.Fatal(err)
	}
}

func TestIssue_UnknownClient(t *testing.T) {
	s := newServer()
	if _, err := s.IssueClientCredentials("missing", "x", nil); err != ErrInvalidClient {
		t.Fatal(err)
	}
}

func TestIssue_BadScope(t *testing.T) {
	s := newServer()
	s.Register(&Client{ID: "c1", Secret: []byte("s1"), Scopes: []string{"read"}})
	if _, err := s.IssueClientCredentials("c1", "s1", []string{"admin"}); err != ErrInvalidScope {
		t.Fatal(err)
	}
}

func TestVerifyToken(t *testing.T) {
	s := newServer()
	s.Register(&Client{ID: "c1", Secret: []byte("s1"), Scopes: []string{"read"}, Tenant: "t1"})
	r, _ := s.IssueClientCredentials("c1", "s1", []string{"read"})
	claims, err := s.VerifyToken(r.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != "c1" {
		t.Fatal("sub")
	}
	if claims["tid"] != "t1" {
		t.Fatal("tid")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := New("https://example.com", "k").WithTime(func() time.Time { return now })
	s.Register(&Client{ID: "c1", Secret: []byte("s1"), AccessTTL: time.Second})
	r, _ := s.IssueClientCredentials("c1", "s1", nil)
	now = now.Add(2 * time.Second)
	if _, err := s.VerifyToken(r.AccessToken); err == nil {
		t.Fatal("expected expiry")
	}
}

func TestVerifyToken_BadSig(t *testing.T) {
	s := newServer()
	s.Register(&Client{ID: "c1", Secret: []byte("s1")})
	r, _ := s.IssueClientCredentials("c1", "s1", nil)
	bad := r.AccessToken + "x"
	if _, err := s.VerifyToken(bad); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyToken_BadFormat(t *testing.T) {
	s := newServer()
	if _, err := s.VerifyToken("not-a-token"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSplitN(t *testing.T) {
	parts := splitN("a.b.c.d", '.', 3)
	if len(parts) != 3 || parts[0] != "a" || parts[1] != "b" || parts[2] != "c.d" {
		t.Fatalf("got: %v", parts)
	}
}

func TestJoinScopes(t *testing.T) {
	if joinScopes([]string{"a", "b"}) != "a b" {
		t.Fatal("join")
	}
	if joinScopes(nil) != "" {
		t.Fatal("empty")
	}
}
