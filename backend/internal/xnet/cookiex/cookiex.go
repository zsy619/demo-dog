// Package cookiex 提供 Cookie 头解析与序列化辅助。
package cookiex

import (
	"net/http"
	"strings"
)

// Parse 解析 Cookie 头字符串为 map。
func Parse(header string) map[string]string {
	out := make(map[string]string)
	if header == "" {
		return out
	}
	for _, p := range strings.Split(header, ";") {
		p = strings.TrimSpace(p)
		eq := strings.IndexByte(p, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(p[:eq])
		v := strings.TrimSpace(p[eq+1:])
		out[k] = v
	}
	return out
}

// Serialize 把 map 序列化为 Cookie 头字符串。
func Serialize(kv map[string]string) string {
	var b strings.Builder
	first := true
	for k, v := range kv {
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
	}
	return b.String()
}

// GetCookie 从 Request 中获取某个 Cookie 值。
func GetCookie(r *http.Request, name string) (string, bool) {
	c, err := r.Cookie(name)
	if err != nil {
		return "", false
	}
	return c.Value, true
}

// SetCookie 设置一个 Cookie 到 ResponseWriter。
func SetCookie(w http.ResponseWriter, name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// DeleteCookie 通过设置过期 Cookie 来删除。
func DeleteCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:   name,
		Value:  "",
		MaxAge: -1,
		Path:   path,
	})
}

// MustParse 解析单个 Set-Cookie 头。
func MustParse(setCookie string) (*http.Cookie, error) {
	return http.ParseSetCookie(setCookie)
}
