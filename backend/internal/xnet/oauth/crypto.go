package oauth

// crypto.go:JWT 签名/校验 + 内部辅助。

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// signToken 用 HS256 签名 claims,返回紧凑型 JWT。
func (s *Server) signToken(claims map[string]any) (string, error) {
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	hp := base64.RawURLEncoding.EncodeToString(hb)
	cp := base64.RawURLEncoding.EncodeToString(cb)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(hp + "." + cp))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hp + "." + cp + "." + sig, nil
}

// verifyToken 解析并校验 JWT 签名,返回 claims。
func (s *Server) verifyToken(token string) (map[string]any, error) {
	parts := splitN(token, '.', 3)
	if len(parts) != 3 {
		return nil, errors.New("bad token")
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(want)) {
		return nil, errors.New("bad signature")
	}
	cb, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(cb, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// newOpaque 生成 32 字节随机不透明字符串。
func newOpaque() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// joinScopes 把 scopes 用空格连接。
func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	out := ""
	for i, sc := range scopes {
		if i > 0 {
			out += " "
		}
		out += sc
	}
	return out
}

// splitN 按 sep 把 s 拆成 n 段(最后一段包含尾部所有内容)。
func splitN(s string, sep byte, n int) []string {
	out := make([]string, 0, n)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
			if len(out) == n-1 {
				break
			}
		}
	}
	out = append(out, s[start:])
	return out
}

// String 把字节切片格式化为字符串(用于错误消息)。
func String(b []byte) string { return fmt.Sprintf("%s", b) }
