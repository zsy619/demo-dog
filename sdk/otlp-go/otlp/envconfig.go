// EnvConfig reads the DOG_* environment variables into a Config
// struct that the caller can hand to otlp.New.
//
// Recognised variables:
//
//	DOG_ENDPOINT  collector base URL (default http://localhost:18080)
//	DOG_API_KEY   bearer token registered on the collector
//	DOG_TENANT    tenant id attached to every record
//
// Empty / unset variables fall back to the defaults so a missing
// env var never silently disables telemetry.

package otlp

import (
	"os"
	"strings"
)

// Config captures every env-driven knob the SDK exposes. Pass it to
// otlp.New via the helper FromEnv when you do not want to compose
// options yourself.
type Config struct {
	Endpoint string
	APIKey   string
	Tenant   string
}

// LoadEnv reads DOG_* variables. Missing values are returned as the
// defaults; missing variables do NOT cause an error because the SDK
// is designed to start in any environment.
func LoadEnv() Config {
	return Config{
		Endpoint: getenv("DOG_ENDPOINT", "http://localhost:18080"),
		APIKey:   strings.TrimSpace(os.Getenv("DOG_API_KEY")),
		Tenant:   strings.TrimSpace(os.Getenv("DOG_TENANT")),
	}
}

// FromEnv is a convenience wrapper that calls New with the env values
// plus the supplied options. Any non-zero options are appended after
// the env-derived ones so they take precedence.
func FromEnv(opts ...SDKOption) (*SDK, error) {
	c := LoadEnv()
	all := []SDKOption{
		WithAuthToken(c.APIKey),
		WithTenant(c.Tenant),
	}
	all = append(all, opts...)
	return New(c.Endpoint, all...)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
