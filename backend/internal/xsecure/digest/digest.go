// Package digest 提供 HTTP Digest 鉴权的简化实现。
package digest

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"strings"
)

// Params 是客户端解析后的 Digest 参数。
type Params struct {
	Username  string
	Realm     string
	Nonce     string
	URI       string
	Response  string
	Algorithm string
	Qop       string
	NC        string
	Cnonce    string
}

// ErrBadFormat 表示 Header 格式错误。
var ErrBadFormat = errors.New("digest: 格式错误")

// Parse 解析 "Authorization: Digest ..." 头。
func Parse(authHeader string) (Params, error) {
	if !strings.HasPrefix(authHeader, "Digest ") {
		return Params{}, ErrBadFormat
	}
	s := strings.TrimPrefix(authHeader, "Digest ")
	p := Params{Algorithm: "MD5"}
	// 按 , 分割，键="值"
	parts := splitComma(s)
	for _, kv := range parts {
		kv = strings.TrimSpace(kv)
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		k := kv[:eq]
		v := strings.Trim(kv[eq+1:], "\"")
		switch k {
		case "username":
			p.Username = v
		case "realm":
			p.Realm = v
		case "nonce":
			p.Nonce = v
		case "uri":
			p.URI = v
		case "response":
			p.Response = v
		case "algorithm":
			p.Algorithm = v
		case "qop":
			p.Qop = v
		case "nc":
			p.NC = v
		case "cnonce":
			p.Cnonce = v
		}
	}
	if p.Username == "" || p.Nonce == "" || p.Response == "" {
		return p, ErrBadFormat
	}
	return p, nil
}

// CheckResponse 校验客户端响应；password 是用户密码，method 是 HTTP 方法。
func CheckResponse(p Params, method, password string) bool {
	ha1 := md5Hash(p.Username + ":" + p.Realm + ":" + password)
	ha2 := md5Hash(method + ":" + p.URI)
	var resp string
	if p.Qop == "auth" || p.Qop == "auth-int" {
		resp = md5Hash(ha1 + ":" + p.Nonce + ":" + p.NC + ":" + p.Cnonce + ":" + p.Qop + ":" + ha2)
	} else {
		resp = md5Hash(ha1 + ":" + p.Nonce + ":" + ha2)
	}
	return resp == p.Response
}

// Challenge 生成 WWW-Authenticate 头。
func Challenge(realm, nonce string, withQop bool) string {
	if withQop {
		return `Digest realm="` + realm + `", nonce="` + nonce + `", qop="auth", algorithm=MD5`
	}
	return `Digest realm="` + realm + `", nonce="` + nonce + `", algorithm=MD5`
}

func md5Hash(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func splitComma(s string) []string {
	out := []string{}
	cur := ""
	inQ := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQ = !inQ
			cur += string(c)
		case c == ',' && !inQ:
			out = append(out, cur)
			cur = ""
		default:
			cur += string(c)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
