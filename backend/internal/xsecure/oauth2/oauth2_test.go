package oauth2

import (
	"errors"
	"strings"
	"testing"
)

func TestNewVerifier(t *testing.T) {
	v, err := NewVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if len(v.CodeVerifier) == 0 || len(v.CodeChallenge) == 0 {
		t.Fatal("verifier")
	}
	if v.Method != "S256" {
		t.Fatal("method")
	}
}

func TestNewState(t *testing.T) {
	s, err := NewState()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 {
		t.Fatal("state")
	}
}

func TestAuthorizeURL(t *testing.T) {
	cfg := Config{ClientID: "c", AuthURL: "https://a.com/auth", RedirectURL: "https://app/cb", Scopes: []string{"openid"}}
	v, _ := NewVerifier()
	state, _ := NewState()
	u := AuthorizeURL(cfg, state, v)
	if !strings.Contains(u, "client_id=c") || !strings.Contains(u, "code_challenge=") || !strings.Contains(u, "state="+state) {
		t.Fatal("url")
	}
}

func TestTokenRequest(t *testing.T) {
	tr := TokenRequest{GrantType: "authorization_code", Code: "abc", RedirectURI: "https://app/cb", CodeVerifier: "v"}
	form := tr.Form("c")
	if !strings.Contains(form, "grant_type=authorization_code") || !strings.Contains(form, "code=abc") {
		t.Fatal("form")
	}
}

func TestTokenRequestRefresh(t *testing.T) {
	tr := TokenRequest{GrantType: "refresh_token", RefreshToken: "x"}
	form := tr.Form("c")
	if !strings.Contains(form, "refresh_token=x") {
		t.Fatal("refresh")
	}
}

func TestValidate(t *testing.T) {
	if err := ValidateCallback("", "s", "s"); !errors.Is(err, ErrEmptyCode) {
		t.Fatal("empty")
	}
	if err := ValidateCallback("c", "a", "b"); !errors.Is(err, ErrBadState) {
		t.Fatal("state")
	}
	if err := ValidateCallback("c", "a", "a"); err != nil {
		t.Fatal("ok")
	}
}

func TestSanitize(t *testing.T) {
	cfg := Config{ClientID: "c"}
	if !strings.Contains(cfg.Sanitize(), "c") {
		t.Fatal("san")
	}
}
