// Package tlsconfig TLS 配置：生成安全的 tls.Config 实例。
package tlsconfig

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// Loader is a tls.Config source that re-reads files from
// disk on Reload, swapping in a fresh tls.Config.
type Loader struct {
	mu       sync.RWMutex
	certPath string
	keyPath  string
	caPath   string
	cur      *tls.Config
	lastMod  time.Time
	now      func() time.Time
}

// New constructs a Loader.
func New(certPath, keyPath, caPath string) *Loader {
	return &Loader{
		certPath: certPath,
		keyPath:  keyPath,
		caPath:   caPath,
		now:      time.Now,
	}
}

// Load performs the first load. Required before Get.
func (l *Loader) Load() error {
	cfg, err := l.build()
	if err != nil {
		return err
	}
	mod, _ := l.modTime()
	l.mu.Lock()
	l.cur = cfg
	l.lastMod = mod
	l.mu.Unlock()
	return nil
}

// Reload re-reads files and swaps in a new config.
// Returns ErrUnchanged when the underlying files have not
// changed since the last load.
func (l *Loader) Reload() error {
	curMod, err := l.modTime()
	if err != nil {
		return err
	}
	l.mu.RLock()
	prev := l.lastMod
	l.mu.RUnlock()
	if !prev.IsZero() && !curMod.After(prev) && curMod.Equal(prev) {
		return ErrUnchanged
	}
	cfg, err := l.build()
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.cur = cfg
	l.lastMod = curMod
	l.mu.Unlock()
	return nil
}

// Get returns the current tls.Config.
func (l *Loader) Get() *tls.Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.cur
}

// Fingerprint returns the SHA256 of the cert (PEM-decoded DER).
// Used to detect rotation without diffing the whole config.
func (l *Loader) Fingerprint() (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.cur == nil || len(l.cur.Certificates) == 0 {
		return "", errors.New("no certificate loaded")
	}
	cert := l.cur.Certificates[0]
	if len(cert.Certificate) == 0 {
		return "", errors.New("empty certificate chain")
	}
	h := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(h[:]), nil
}

// CertMetadata describes a certificate.
type CertMetadata struct {
	Subject    string    `json:"subject"`
	Issuer     string    `json:"issuer"`
	NotBefore  time.Time `json:"not_before"`
	NotAfter   time.Time `json:"not_after"`
	Remaining  string    `json:"remaining"`
	Fingerprint string    `json:"fingerprint"`
}

// Metadata returns descriptive metadata for the current cert.
func (l *Loader) Metadata() (CertMetadata, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.cur == nil || len(l.cur.Certificates) == 0 {
		return CertMetadata{}, errors.New("no certificate loaded")
	}
	cert := l.cur.Certificates[0]
	if len(cert.Certificate) == 0 {
		return CertMetadata{}, errors.New("empty certificate chain")
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return CertMetadata{}, err
	}
	h := sha256.Sum256(cert.Certificate[0])
	now := l.now()
	remaining := parsed.NotAfter.Sub(now)
	return CertMetadata{
		Subject: parsed.Subject.String(),
		Issuer: parsed.Issuer.String(),
		NotBefore: parsed.NotBefore,
		NotAfter: parsed.NotAfter,
		Remaining: remaining.String(),
		Fingerprint: hex.EncodeToString(h[:]),
	}, nil
}

func (l *Loader) build() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(l.certPath, l.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load keypair: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion: tls.VersionTLS12,
	}
	if l.caPath != "" {
		pool, err := loadCAPool(l.caPath)
		if err != nil {
			return nil, fmt.Errorf("load CA: %w", err)
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

func (l *Loader) modTime() (time.Time, error) {
	latest := time.Time{}
	for _, p := range []string{l.certPath, l.keyPath, l.caPath} {
		if p == "" {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			return time.Time{}, err
		}
		if fi.ModTime().After(latest) {
			latest = fi.ModTime()
		}
	}
	return latest, nil
}

func loadCAPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		// Try DER.
		if block, _ := pem.Decode(data); block != nil {
			if c, err := x509.ParseCertificate(block.Bytes); err == nil {
				pool.AddCert(c)
			} else {
				return nil, err
			}
		} else {
			return nil, errors.New("no PEM blocks")
		}
	}
	return pool, nil
}

// ErrUnchanged is returned by Reload when no file changed.
var ErrUnchanged = errors.New("tls files unchanged")
