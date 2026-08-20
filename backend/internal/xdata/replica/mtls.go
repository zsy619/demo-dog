package replica

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// mTLSConfig configures mutual-TLS for the /replica endpoint.
//
// Both sides authenticate: the client (follower) presents a cert
// the primary verifies against a CA bundle, and the primary
// presents its own cert the follower verifies.
//
// Wire format:
//
//	primary  --tls-cert=primary.pem --tls-key=primary.key --ca-bundle=ca.pem
//	follower --tls-cert=follower.pem --tls-key=follower.key --ca-bundle=ca.pem
type mTLSConfig struct {
	Cert     []byte
	Key      []byte
	CABundle []byte
	ClientCAs []byte
	RequireClientCert bool
	MinVersion uint16
}

// ServerTLSConfig returns a *tls.Config for the primary listener.
func (m *mTLSConfig) ServerTLSConfig() (*tls.Config, error) {
	if len(m.Cert) == 0 || len(m.Key) == 0 {
		return nil, errors.New("server cert/key required")
	}
	c, err := tls.X509KeyPair(m.Cert, m.Key)
	if err != nil {
		return nil, fmt.Errorf("server keypair: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{c},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.NoClientCert,
	}
	if m.MinVersion != 0 {
		cfg.MinVersion = m.MinVersion
	}
	if m.RequireClientCert || len(m.ClientCAs) > 0 {
		pool, err := buildCertPool(m.ClientCAs)
		if err != nil {
			return nil, fmt.Errorf("client CA pool: %w", err)
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

// ClientTLSConfig 返回 follower HTTP 客户端使用的 *tls.Config。
func (m *mTLSConfig) ClientTLSConfig(serverName string) (*tls.Config, error) {
	if len(m.Cert) == 0 || len(m.Key) == 0 {
		return nil, errors.New("client cert/key required")
	}
	c, err := tls.X509KeyPair(m.Cert, m.Key)
	if err != nil {
		return nil, fmt.Errorf("client keypair: %w", err)
	}
	pool, err := buildCertPool(m.CABundle)
	if err != nil {
		return nil, fmt.Errorf("CA pool: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{c},
		RootCAs:      pool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func buildCertPool(pemData []byte) (*x509.CertPool, error) {
	if len(pemData) == 0 {
		return nil, errors.New("CA bundle is empty")
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, errors.New("no certificates in PEM data")
	}
	return pool, nil
}

// NewMTLSClient 返回一个为 follower 出示证书的 *http.Client
// client cert on every request.
func NewMTLSClient(cfg *tls.Config, timeout time.Duration) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: cfg,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
	}
	return &http.Client{Transport: tr, Timeout: timeout}
}

// CertFingerprint 以十六进制返回证书的 SHA-256 指纹。
func CertFingerprint(cert *x509.Certificate) string {
	h := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(h[:])
}

// CertSubjectCN 返回 CN（或 SPIFFE 的首个 SAN DNS / IP）。
func CertSubjectCN(cert *x509.Certificate) string {
	if cert.Subject.CommonName != "" {
		return cert.Subject.CommonName
	}
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	if len(cert.IPAddresses) > 0 {
		return cert.IPAddresses[0].String()
	}
	return ""
}

// ParsedCert 是对端证书的线上稳定形式。
type ParsedCert struct {
	Subject     pkix.Name
	Issuer      pkix.Name
	NotBefore   time.Time
	NotAfter    time.Time
	DNSNames    []string
	IPAddrs     []net.IP
	Fingerprint string
	Serial      string
}

// PeerCertFromRequest 从 in-flight 请求中提取对端证书。
func PeerCertFromRequest(r *http.Request) *x509.Certificate {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return nil
	}
	return r.TLS.PeerCertificates[0]
}

// CertAllowedBySAN 报告证书是否被允许用于
// expected hostname. Looks at DNSNames and IPAddresses in the SAN.
func CertAllowedBySAN(cert *x509.Certificate, hostname string) bool {
	if cert == nil {
		return false
	}
	for _, n := range cert.DNSNames {
		if n == hostname {
			return true
		}
	}
	ip := net.ParseIP(hostname)
	if ip != nil {
		for _, a := range cert.IPAddresses {
			if a.Equal(ip) {
				return true
			}
		}
	}
	return false
}

// SplitPEM 将 PEM 包拆分为单独的证书。
func SplitPEM(data []byte) [][]byte {
	out := [][]byte{}
	current := []byte{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "-----") && len(strings.TrimSpace(line)) == 5 {
			current = append(current, []byte(line+"\n")...)
			if len(current) > 0 {
				out = append(out, current)
				current = []byte{}
			}
			continue
		}
		current = append(current, []byte(line+"\n")...)
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

// Allow 用于抑制 mtLS sync.Mutex 的未使用警告。
var _ = sync.Mutex{}
