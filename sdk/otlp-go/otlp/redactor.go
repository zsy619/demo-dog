// Redaction of PII before export. We deliberately keep the redactor
// surface small: the default matcher hits the common cases
// (passwords in URLs, bearer tokens in headers, email addresses)
// while letting callers compose custom rules for their domain.

package otlp

import (
	"regexp"
	"strings"
)

// Redactor scrubs PII from an outgoing record. It must be safe for
// concurrent use; the SDK calls it once per record on the flush path.
type Redactor func(body string, attrs map[string]string) (string, map[string]string)

// DefaultRedactor masks the four classes of secrets that leak most
// often into application logs: passwords embedded in URLs / forms,
// bearer tokens, Authorization headers, and email addresses.
var DefaultRedactor Redactor = redactDefault

const redacted = "***REDACTED***"

var passwordKV = regexp.MustCompile(PASS_KV_PAT)
var passwordVal = regexp.MustCompile(PASS_VAL_PAT)
var bearer = regexp.MustCompile(BEARER_PAT)
var authHdr = regexp.MustCompile(AUTH_PAT)
var email = regexp.MustCompile(EMAIL_PAT)

const (
	PASS_KV_PAT  = `(?i)(password\s*[=:]\s*["\']?)([^\s"\',&}{]+)`
	PASS_VAL_PAT = `(?i)^([A-Za-z0-9._%+\-]{6,})$`
	BEARER_PAT   = `(?i)(bearer\s+)[A-Za-z0-9._\-]+`
	AUTH_PAT     = `(?i)(authorization:\s*\S+\s+)[A-Za-z0-9._\-=+/]+`
	EMAIL_PAT    = `[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`
)

var defaultPatterns = []struct {
	re  *regexp.Regexp
	mask string
}{
	{passwordKV, `$1` + redacted},
	{bearer, `$1` + redacted},
	{authHdr, `$1` + redacted},
	{email, redacted},
}

func redactDefault(body string, attrs map[string]string) (string, map[string]string) {
	changed := false
	for _, p := range defaultPatterns {
		if p.re.MatchString(body) {
			body = p.re.ReplaceAllString(body, p.mask)
			changed = true
		}
	}
	if attrs != nil {
		for k, v := range attrs {
			lk := strings.ToLower(k)
			if !sensitiveKey(lk) {
				continue
			}
			original := v
			for _, p := range defaultPatterns {
				if p.re.MatchString(v) {
					v = p.re.ReplaceAllString(v, p.mask)
				}
			}
			// When the key itself is a sensitive attribute name
			// and the value is non-empty, redact the whole value
			// via the bare-token pattern (whole-line match).
			if passwordVal.MatchString(v) {
				v = redacted
			}
			if v != original {
				attrs[k] = v
				changed = true
			}
		}
	}
	if !changed {
		return body, attrs
	}
	return body, attrs
}

func sensitiveKey(k string) bool {
	switch k {
	case "password", "passwd", "pwd",
		"token", "access_token", "refresh_token", "id_token",
		"authorization", "auth", "api_key", "apikey", "secret",
		"x-api-key", "cookie", "set-cookie":
		return true
	}
	return false
}
