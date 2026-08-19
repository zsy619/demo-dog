// Package certmini 提供 PEM 块解析与简单自签名生成辅助。
package certmini

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"
)

// ParsePEM 解析 PEM 数据，返回 type 和 bytes。
func ParsePEM(b []byte) (string, []byte, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return "", nil, errors.New("certmini: 非法 PEM")
	}
	return block.Type, block.Bytes, nil
}

// EncodePEM 把 type+bytes 编码为 PEM。
func EncodePEM(typ string, b []byte) []byte {
	out := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: b})
	if out == nil {
		return nil
	}
	return out
}

// SelfSigned 生成一个自签名证书与私钥（PEM 格式）。
func SelfSigned(commonName string, days int) (certPEM, keyPEM []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	if days < 1 {
		days = 365
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	certPEM = EncodePEM("CERTIFICATE", der)
	keyPEM = EncodePEM("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(priv))
	return certPEM, keyPEM, nil
}
