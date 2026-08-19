package digest

import "testing"

func TestParse(t *testing.T) {
	h := `Digest username="alice", realm="r", nonce="n", uri="/x", response="r1"`
	p, err := Parse(h)
	if err != nil {
		t.Fatal(err)
	}
	if p.Username != "alice" || p.Realm != "r" || p.Nonce != "n" || p.URI != "/x" || p.Response != "r1" {
		t.Fatal("parse", p)
	}
}

func TestParse_BadPrefix(t *testing.T) {
	if _, err := Parse("Basic foo"); err == nil {
		t.Fatal("prefix")
	}
}

func TestParse_Missing(t *testing.T) {
	if _, err := Parse("Digest foo=bar"); err == nil {
		t.Fatal("missing")
	}
}

func TestRoundTrip(t *testing.T) {
	username := "alice"
	realm := "testrealm"
	password := "secret"
	nonce := "abc123"
	uri := "/x"
	method := "GET"
	ha1 := md5Hash_(username + ":" + realm + ":" + password)
	ha2 := md5Hash_(method + ":" + uri)
	resp := md5Hash_(ha1 + ":" + nonce + ":" + ha2)
	h := `Digest username="alice", realm="testrealm", nonce="abc123", uri="/x", response="` + resp + `"`
	p, err := Parse(h)
	if err != nil {
		t.Fatal(err)
	}
	if !CheckResponse(p, method, password) {
		t.Fatal("check")
	}
}

func TestCheckResponse_NoQop(t *testing.T) {
	username := "bob"
	realm := "r"
	password := "p"
	nonce := "n"
	uri := "/"
	method := "GET"
	ha1 := md5Hash_(username + ":" + realm + ":" + password)
	ha2 := md5Hash_(method + ":" + uri)
	resp := md5Hash_(ha1 + ":" + nonce + ":" + ha2)
	p := Params{Username: username, Realm: realm, Nonce: nonce, URI: uri, Response: resp}
	if !CheckResponse(p, method, password) {
		t.Fatal("noqop")
	}
}

func TestCheckResponse_QopAuth(t *testing.T) {
	ha1 := md5Hash_("a:r:p")
	ha2 := md5Hash_("GET:/x")
	resp := md5Hash_(ha1 + ":n:0001:c:auth:" + ha2)
	p := Params{Username: "a", Realm: "r", Nonce: "n", URI: "/x", Response: resp, Qop: "auth", NC: "0001", Cnonce: "c"}
	if !CheckResponse(p, "GET", "p") {
		t.Fatal("qop")
	}
}

func TestChallenge(t *testing.T) {
	c := Challenge("r", "n", false)
	if c == "" || c[0] != 'D' {
		t.Fatal("challenge")
	}
}

func md5Hash_(s string) string { return md5Hash(s) }
