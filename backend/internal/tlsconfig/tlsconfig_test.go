package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeCertPair(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cf, _ := os.Create(certPath)
	pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	cf.Close()
	kf, _ := os.Create(keyPath)
	b, _ := x509.MarshalECPrivateKey(key)
	pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	kf.Close()
	return
}

func writeCAPEM(t *testing.T, dir string) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{CommonName: "ca"},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().Add(time.Hour),
		IsCA: true,
		KeyUsage: x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	p := filepath.Join(dir, "ca.pem")
	f, _ := os.Create(p)
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	f.Close()
	return p
}

func TestLoad_Success(t *testing.T) {
	cp, kp := writeCertPair(t)
	l := New(cp, kp, "")
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	if l.Get() == nil {
		t.Fatal("nil config")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	l := New("/no/such/cert", "/no/such/key", "")
	if err := l.Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestReload_DetectsChange(t *testing.T) {
	cp, kp := writeCertPair(t)
	l := New(cp, kp, "")
	l.Load()
	fp1, _ := l.Fingerprint()
	// Rewrite with a new cert.
	time.Sleep(10 * time.Millisecond)
	cp2, kp2 := writeCertPair(t)
	l2 := New(cp2, kp2, "")
	l2.Load()
	// Mutate files so modtime changes.
	l.certPath = cp2
	l.keyPath = kp2
	if err := l.Reload(); err != nil {
		t.Fatal(err)
	}
	fp2, _ := l.Fingerprint()
	if fp1 == fp2 {
		t.Fatal("fingerprint should differ")
	}
}

func TestReload_Unchanged(t *testing.T) {
	cp, kp := writeCertPair(t)
	l := New(cp, kp, "")
	l.Load()
	err := l.Reload()
	if err != ErrUnchanged {
		t.Fatalf("expected ErrUnchanged, got %v", err)
	}
}

func TestMetadata(t *testing.T) {
	cp, kp := writeCertPair(t)
	l := New(cp, kp, "")
	l.Load()
	m, err := l.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if m.Subject == "" {
		t.Fatal("no subject")
	}
	if m.Fingerprint == "" {
		t.Fatal("no fingerprint")
	}
	if m.NotAfter.IsZero() {
		t.Fatal("no not-after")
	}
}

func TestFingerprint_AfterLoadNotLoaded(t *testing.T) {
	l := New("", "", "")
	if _, err := l.Fingerprint(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_WithCA(t *testing.T) {
	cp, kp := writeCertPair(t)
	caDir := t.TempDir()
	ca := writeCAPEM(t, caDir)
	l := New(cp, kp, ca)
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	cfg := l.Get()
	if cfg.ClientCAs == nil {
		t.Fatal("CA pool not loaded")
	}
	if cfg.ClientAuth != 4 {
		t.Fatalf("client auth: %d", cfg.ClientAuth)
	}
}

func TestLoad_BadCA(t *testing.T) {
	cp, kp := writeCertPair(t)
	l := New(cp, kp, "/no/such/ca")
	if err := l.Load(); err == nil {
		t.Fatal("expected error")
	}
}
