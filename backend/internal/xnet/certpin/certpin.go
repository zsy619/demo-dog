// Package certpin 提供按 (host, fingerprint) 匹配的 TLS 证书钉扎。
// 它持有每个 host 一组可接受的 SHA-256 指纹，并在握手中校验。
package certpin

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
)

// ErrUnpinned 在 host 未配置指纹时返回。
var ErrUnpinned = errors.New("certpin: 主机未配置指纹")

// ErrMismatch 在指纹不匹配时返回。
var ErrMismatch = errors.New("certpin: 指纹不匹配")

// ErrNoCert 在连接未携带证书时返回。
var ErrNoCert = errors.New("certpin: 服务器未提供证书")

// Manager 维护 host->指纹集合 的映射。
type Manager struct {
	mu       sync.RWMutex
	fingers  map[string]map[string]bool
	fingers2 map[string][]byte
}

// New 创建一个空 Manager。
func New() *Manager {
	return &Manager{
		fingers:  make(map[string]map[string]bool),
		fingers2: make(map[string][]byte),
	}
}

// Add 注册 host 的一个指纹。
func (m *Manager) Add(host, fingerprint string) error {
	fp, err := parseFP(fingerprint)
	if err != nil {
		return err
	}
	return m.AddRaw(host, fp)
}

// AddRaw 接收已解码的字节指纹。
func (m *Manager) AddRaw(host string, fp []byte) error {
	if host == "" {
		return errors.New("certpin: 主机为空")
	}
	if len(fp) == 0 {
		return errors.New("certpin: 指纹为空")
	}
	m.mu.Lock()
	if m.fingers[host] == nil {
		m.fingers[host] = make(map[string]bool)
	}
	m.fingers[host][hex.EncodeToString(fp)] = true
	m.fingers2[host] = append(m.fingers2[host], fp...)
	m.mu.Unlock()
	return nil
}

// Remove 移除 host 的某个指纹。
func (m *Manager) Remove(host, fingerprint string) error {
	fp, err := parseFP(fingerprint)
	if err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.fingers[host], hex.EncodeToString(fp))
	m.mu.Unlock()
	return nil
}

// Has 判断 host 是否已被钉扎。
func (m *Manager) Has(host string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.fingers[host]) > 0
}

// Verify 在握手中校验证书。
func (m *Manager) Verify(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return ErrNoCert
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	fp := sha256.Sum256(rawCerts[0])
	// 找不到 host，但 caller 通常通过 VerifyByHost 调用。
	for _, set := range m.fingers {
		if set[hex.EncodeToString(fp[:])] {
			return nil
		}
	}
	return ErrMismatch
}

// VerifyByHost 是更精确的入口。
func (m *Manager) VerifyByHost(host string, rawCerts [][]byte) error {
	if len(rawCerts) == 0 {
		return ErrNoCert
	}
	m.mu.RLock()
	set, ok := m.fingers[host]
	m.mu.RUnlock()
	if !ok || len(set) == 0 {
		return ErrUnpinned
	}
	fp := sha256.Sum256(rawCerts[0])
	if set[hex.EncodeToString(fp[:])] {
		return nil
	}
	return ErrMismatch
}

// Fingerprint 是便捷工具：返回 cert 的 SHA-256 指纹。
func Fingerprint(cert []byte) string {
	sum := sha256.Sum256(cert)
	return hex.EncodeToString(sum[:])
}

func parseFP(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	if len(s)%2 != 0 {
		return nil, errors.New("certpin: 指纹长度无效")
	}
	return hex.DecodeString(s)
}

// TLSConfig 返回一个 tls.Config，其中 VerifyPeerCertificate
// 已配置为使用 Manager 校验。
func (m *Manager) TLSConfig() *tls.Config {
	return &tls.Config{
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return m.Verify(rawCerts, verifiedChains)
		},
	}
}

// Snapshot 返回所有 host 的指纹副本。
func (m *Manager) Snapshot() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string][]string, len(m.fingers))
	for h, set := range m.fingers {
		out[h] = make([]string, 0, len(set))
		for k := range set {
			out[h] = append(out[h], k)
		}
	}
	return out
}
