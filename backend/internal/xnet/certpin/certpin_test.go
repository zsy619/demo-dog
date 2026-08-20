package certpin

import (
	"crypto/sha256"
	"errors"
	"testing"
)

func TestAddRemove(t *testing.T) {
	m := New()
	fp := sha256.Sum256([]byte("cert"))
	h := hexEncode(fp[:])
	if err := m.Add("example.com", h); err != nil {
		t.Fatal(err)
	}
	if !m.Has("example.com") {
		t.Fatal("应存在")
	}
	if err := m.Remove("example.com", h); err != nil {
		t.Fatal(err)
	}
	if m.Has("example.com") {
		t.Fatal("应被移除")
	}
}

func TestAdd_BadFP(t *testing.T) {
	m := New()
	if err := m.Add("h", "zz"); err == nil {
		t.Fatal("应拒绝非法指纹")
	}
}

func TestVerify_Match(t *testing.T) {
	m := New()
	cert := []byte("hello")
	fp := sha256.Sum256(cert)
	h := hexEncode(fp[:])
	m.Add("example.com", h)
	rawCerts := [][]byte{cert}
	if err := m.VerifyByHost("example.com", rawCerts); err != nil {
		t.Fatal(err)
	}
}

func TestVerify_Mismatch(t *testing.T) {
	m := New()
	m.Add("example.com", hexEncode([]byte("some-fp")))
	if err := m.VerifyByHost("example.com", [][]byte{[]byte("other-cert")}); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
}

func TestVerify_Unpinned(t *testing.T) {
	m := New()
	if err := m.VerifyByHost("missing.com", [][]byte{[]byte("x")}); !errors.Is(err, ErrUnpinned) {
		t.Fatal(err)
	}
}

func TestVerify_NoCert(t *testing.T) {
	m := New()
	if err := m.VerifyByHost("h", nil); !errors.Is(err, ErrNoCert) {
		t.Fatal(err)
	}
}

func TestFingerprint(t *testing.T) {
	if Fingerprint([]byte("a")) == "" {
		t.Fatal("fingerprint")
	}
}

func TestTLSConfig(t *testing.T) {
	m := New()
	cfg := m.TLSConfig()
	if cfg.VerifyPeerCertificate == nil {
		t.Fatal("hook 应设置")
	}
}

func TestSnapshot(t *testing.T) {
	m := New()
	m.Add("a", hexEncode([]byte("x")))
	snap := m.Snapshot()
	if len(snap) != 1 {
		t.Fatal("snap")
	}
}

func hexEncode(b []byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hex[c>>4]
		out[i*2+1] = hex[c&0x0f]
	}
	return string(out)
}
