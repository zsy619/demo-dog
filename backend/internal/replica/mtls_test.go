package replica

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// generateTestCA creates a self-signed CA certificate.
func generateTestCA(t *testing.T) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	parsed, _ := x509.ParseCertificate(der)
	return parsed, pemBytes, key
}

// generateLeaf creates a leaf cert signed by the CA.
func generateLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey,
	cn string, dnsNames []string, ipAddresses []net.IP) (*x509.Certificate, []byte, []byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
		IPAddresses:  ipAddresses,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	parsed, _ := x509.ParseCertificate(der)
	return parsed, pemBytes, keyPEM, key
}

func TestMTLS_ServerConfig_RequiresKeyPair(t *testing.T) {
	m := &mTLSConfig{}
	if _, err := m.ServerTLSConfig(); err == nil {
		t.Fatal("expected error for missing cert/key")
	}
}

func TestMTLS_BuildConfig_ValidPair(t *testing.T) {
	ca, caPEM, caKey := generateTestCA(t)
	_ = ca
	_, leafCert, leafKey, _ := generateLeaf(t, ca, caKey, "follower", nil, nil)
	m := &mTLSConfig{Cert: leafCert, Key: leafKey, ClientCAs: caPEM, RequireClientCert: true}
	cfg, err := m.ServerTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("ClientAuth: %v", cfg.ClientAuth)
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion: %v", cfg.MinVersion)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 cert")
	}
}

func TestMTLS_ClientConfig_RequiresCert(t *testing.T) {
	m := &mTLSConfig{}
	if _, err := m.ClientTLSConfig("localhost"); err == nil {
		t.Fatal("expected error for missing cert/key")
	}
}

func TestMTLS_ClientConfig_BadCABundle(t *testing.T) {
	ca, _, caKey := generateTestCA(t)
	_, leafCert, leafKey, _ := generateLeaf(t, ca, caKey, "follower", nil, nil)
	m := &mTLSConfig{Cert: leafCert, Key: leafKey, CABundle: []byte("not pem")}
	if _, err := m.ClientTLSConfig("primary"); err == nil {
		t.Fatal("expected error for bad CA")
	}
}

func TestMTLS_BuildCertPool_Empty(t *testing.T) {
	if _, err := buildCertPool(nil); err == nil {
		t.Fatal("expected error for empty CA bundle")
	}
}

func TestMTLS_BuildCertPool_Invalid(t *testing.T) {
	if _, err := buildCertPool([]byte("garbage data")); err == nil {
		t.Fatal("expected error for invalid PEM")
	}
}

func TestMTLS_BuildCertPool_Valid(t *testing.T) {
	_, caPEM, _ := generateTestCA(t)
	pool, err := buildCertPool(caPEM)
	if err != nil {
		t.Fatal(err)
	}
	if pool == nil {
		t.Fatal("nil pool")
	}
}

func TestMTLS_CertFingerprint(t *testing.T) {
	ca, _, _ := generateTestCA(t)
	fp := CertFingerprint(ca)
	if len(fp) != 64 {
		t.Fatalf("fingerprint len: %d", len(fp))
	}
	if CertFingerprint(ca) != fp {
		t.Fatal("fingerprint should be deterministic")
	}
}

func TestMTLS_CertSubjectCN(t *testing.T) {
	ca, _, _ := generateTestCA(t)
	cn := CertSubjectCN(ca)
	if cn != "test-ca" {
		t.Fatalf("CN: %s", cn)
	}
}

func TestMTLS_CertAllowedBySAN_DNS(t *testing.T) {
	ca, _, caKey := generateTestCA(t)
	leaf, _, _, _ := generateLeaf(t, ca, caKey, "follower", []string{"node-a.example.com"}, nil)
	if !CertAllowedBySAN(leaf, "node-a.example.com") {
		t.Fatal("should allow SAN match")
	}
	if CertAllowedBySAN(leaf, "other.example.com") {
		t.Fatal("should not allow different name")
	}
}

func TestMTLS_CertAllowedBySAN_IP(t *testing.T) {
	ca, _, caKey := generateTestCA(t)
	leaf, _, _, _ := generateLeaf(t, ca, caKey, "follower", nil, []net.IP{net.ParseIP("10.0.0.5")})
	if !CertAllowedBySAN(leaf, "10.0.0.5") {
		t.Fatal("should allow SAN IP match")
	}
	if CertAllowedBySAN(leaf, "10.0.0.6") {
		t.Fatal("should not allow different IP")
	}
}

func TestMTLS_CertAllowedBySAN_Nil(t *testing.T) {
	if CertAllowedBySAN(nil, "x") {
		t.Fatal("nil cert should not match")
	}
}

func TestMTLS_SplitPEM(t *testing.T) {
	_, caPEM, _ := generateTestCA(t)
	parts := SplitPEM(caPEM)
	if len(parts) == 0 {
		t.Fatal("expected at least one PEM block")
	}
	for _, p := range parts {
		if !strings.Contains(string(p), "BEGIN ") {
			t.Errorf("malformed PEM: %q", string(p))
		}
	}
}

func TestMTLS_NewMTLSClient(t *testing.T) {
	_, caPEM, _ := generateTestCA(t)
	c := NewMTLSClient(&tls.Config{RootCAs: mustPoolMTLS(t, caPEM)}, 5*time.Second)
	if c == nil {
		t.Fatal("nil client")
	}
	if c.Timeout != 5*time.Second {
		t.Fatalf("timeout: %v", c.Timeout)
	}
}

func mustPoolMTLS(t *testing.T, pem []byte) *x509.CertPool {
	t.Helper()
	p, err := buildCertPool(pem)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
