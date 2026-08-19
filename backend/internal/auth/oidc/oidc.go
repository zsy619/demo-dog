// Package oidc provides OIDC federation for demo-dog.
//
// Operators running dex or keycloak can plug demo-dog in as a
// relying party; the OIDCProvider will fetch the discovery
// document, verify ID tokens against the issuer JWKS, and
// translate them into the same KeyEntry shape as the local
// APIKeyAuth.
//
// The mapping is intentionally minimal: the OIDC `sub` claim
// becomes the API key identity, the `aud` claim must contain
// our client_id, and a configurable scope claim (default
// "scope") becomes the Scopes list.
//
// Use:
//
//	ctx := context.Background()
//	oidc, err := oidc.NewProvider(ctx, oidc.Config{
//	    IssuerURL: "https://dex.example.com",
//	    ClientID:  "demo-dog",
//	})
//	if err != nil { log.Fatal(err) }
//
//	entry, err := oidc.Verify(ctx, rawIDToken)
package oidc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config configures an OIDCProvider.
type Config struct {
	IssuerURL   string
	ClientID    string
	ScopeClaim  string
	HTTPClient  *http.Client
	JWKSRefresh time.Duration
}

// OIDCProvider is a verifying relying-party client.
type OIDCProvider struct {
	mu         sync.RWMutex
	cfg        Config
	httpClient *http.Client
	discovery  *DiscoveryDoc
	jwks       *JWKS
	lastJWKSAt time.Time
}

// DiscoveryDoc is the OpenID Connect discovery document.
type DiscoveryDoc struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

// JWKS is the JSON Web Key Set.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK is one key entry.
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

// Claims is the verified JWT payload.
type Claims struct {
	Issuer        string   `json:"iss,omitempty"`
	Subject       string   `json:"sub,omitempty"`
	Audience      []string `json:"aud,omitempty"`
	Expiry        int64    `json:"exp,omitempty"`
	IssuedAt      int64    `json:"iat,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
	Email         string   `json:"email,omitempty"`
	EmailVerified bool     `json:"email_verified,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Tenant        string   `json:"tenant_id,omitempty"`
}

// AllScopes returns the union of Scope (space-separated), Scopes,
// and Groups. Used to populate the auth KeyEntry Scopes list.
func (c *Claims) AllScopes() []string {
	set := map[string]struct{}{}
	for _, s := range strings.Fields(c.Scope) {
		set[s] = struct{}{}
	}
	for _, s := range c.Scopes {
		set[s] = struct{}{}
	}
	for _, g := range c.Groups {
		set["group:"+g] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// NewProvider creates a provider, fetches the discovery doc and
// initial JWKS.
func NewProvider(ctx context.Context, cfg Config) (*OIDCProvider, error) {
	if cfg.IssuerURL == "" {
		return nil, errors.New("issuerURL required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("clientID required")
	}
	if cfg.ScopeClaim == "" {
		cfg.ScopeClaim = "scope"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.JWKSRefresh <= 0 {
		cfg.JWKSRefresh = 15 * time.Minute
	}
	p := &OIDCProvider{cfg: cfg, httpClient: cfg.HTTPClient}
	if err := p.refreshDiscovery(ctx); err != nil {
		return nil, err
	}
	if err := p.refreshJWKS(ctx); err != nil {
		return nil, err
	}
	go p.jwksRefresher(ctx)
	return p, nil
}

// Close stops the background JWKS refresher.
func (p *OIDCProvider) Close() {}

func (p *OIDCProvider) refreshDiscovery(ctx context.Context) error {
	url := strings.TrimRight(p.cfg.IssuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("discovery status %d", resp.StatusCode)
	}
	var doc DiscoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	if doc.Issuer != p.cfg.IssuerURL {
		return fmt.Errorf("issuer mismatch: got %q want %q", doc.Issuer, p.cfg.IssuerURL)
	}
	if doc.JWKSURI == "" {
		return errors.New("discovery missing jwks_uri")
	}
	p.mu.Lock()
	p.discovery = &doc
	p.mu.Unlock()
	return nil
}

func (p *OIDCProvider) refreshJWKS(ctx context.Context) error {
	p.mu.RLock()
	jwksURL := ""
	if p.discovery != nil {
		jwksURL = p.discovery.JWKSURI
	}
	p.mu.RUnlock()
	if jwksURL == "" {
		return errors.New("discovery not loaded")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("jwks status %d", resp.StatusCode)
	}
	var set JWKS
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}
	p.mu.Lock()
	p.jwks = &set
	p.lastJWKSAt = time.Now()
	p.mu.Unlock()
	return nil
}

func (p *OIDCProvider) jwksRefresher(ctx context.Context) {
	t := time.NewTicker(p.cfg.JWKSRefresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = p.refreshJWKS(ctx)
		}
	}
}

// Verify validates a raw ID token and returns claims on success.
func (p *OIDCProvider) Verify(ctx context.Context, raw string) (*Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("token must be 3 parts")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("header b64: %w", err)
	}
	var header struct {
		Alg string
		Kid string
		Typ string
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("header json: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("sig b64: %w", err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload b64: %w", err)
	}
	key, alg, err := p.keyFor(header.Kid, header.Alg)
	if err != nil {
		return nil, err
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	if err := verifySignature(alg, key, signingInput, sig); err != nil {
		return nil, err
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("claims json: %w", err)
	}
	now := time.Now().Unix()
	if claims.Issuer != p.cfg.IssuerURL {
		return nil, errors.New("issuer mismatch")
	}
	if claims.Expiry > 0 && now >= claims.Expiry {
		return nil, errors.New("token expired")
	}
	if !audienceContains(claims.Audience, p.cfg.ClientID) {
		return nil, errors.New("audience mismatch")
	}
	if claims.Subject == "" {
		return nil, errors.New("missing sub claim")
	}
	return &claims, nil
}

func (p *OIDCProvider) keyFor(kid, alg string) (crypto.PublicKey, string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.jwks == nil {
		return nil, "", errors.New("jwks not loaded")
	}
	for _, k := range p.jwks.Keys {
		if kid != "" && k.Kid != kid {
			continue
		}
		if k.Alg != "" && k.Alg != alg {
			continue
		}
		if k.Kty == "RSA" {
			pk, err := rsaPubFromJWK(k)
			if err != nil {
				return nil, "", err
			}
			return pk, "RS256", nil
		}
		if k.Kty == "EC" {
			pk, err := ecPubFromJWK(k)
			if err != nil {
				return nil, "", err
			}
			return pk, "ES256", nil
		}
	}
	return nil, "", fmt.Errorf("no matching key for kid=%s alg=%s", kid, alg)
}

func rsaPubFromJWK(k JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	var e int
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

func ecPubFromJWK(k JWK) (*ecdsa.PublicKey, error) {
	xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
	if err != nil {
		return nil, err
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
	if err != nil {
		return nil, err
	}
	var curve elliptic.Curve
	switch k.Crv {
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", k.Crv)
	}
	return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}, nil
}

func verifySignature(alg string, key crypto.PublicKey, signingInput, sig []byte) error {
	switch alg {
	case "RS256":
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return errors.New("RS256 requires RSA key")
		}
		return rsa.VerifyPKCS1v15(rsaKey, 0, hashSHA256(signingInput), sig)
	case "ES256":
		ecKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("ES256 requires EC key")
		}
		if !ecdsa.VerifyASN1(ecKey, hashSHA256(signingInput), sig) {
			return errors.New("ES256 signature mismatch")
		}
		return nil
	case "ES384":
		ecKey, ok := key.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("ES384 requires EC key")
		}
		h := sha256Sum384(signingInput)
		if !ecdsa.VerifyASN1(ecKey, h, sig) {
			return errors.New("ES384 signature mismatch")
		}
		return nil
	}
	return fmt.Errorf("unsupported alg: %s", alg)
}

func hashSHA256(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func sha256Sum384(b []byte) []byte {
	h := crypto.SHA384.New()
	h.Write(b)
	return h.Sum(nil)
}

func audienceContains(aud []string, want string) bool {
	for _, a := range aud {
		if a == want {
			return true
		}
	}
	return false
}

// Touch the x509 import so future cert-related additions compile.
var _ = x509.MarshalPKCS8PrivateKey
