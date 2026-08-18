// OpenTelemetry semantic conventions for resource attributes.
//
// Per the OTel resource semantic conventions, the SDK automatically
// injects the following attributes when WithAutoResource(true) is set:
//
//   service.name                 (already required)
//   service.version              (from WithServiceVersion)
//   deployment.environment       (from WithDeploymentEnvironment)
//   host.name                    (from WithHostName)
//   process.pid                  (os.Getpid())
//   process.runtime.name         ("go")
//   process.runtime.version      (runtime.Version())
//   process.executable.name      (os.Args[0] basename)
//   host.arch                    (runtime.GOARCH)
//   os.type                      (runtime.GOOS)
//   telemetry.sdk.name            ("otlp-go")
//   telemetry.sdk.language       ("go")
//   telemetry.sdk.version        (Version)
//
// The default is OFF -- you opt in with WithAutoResource(true). This
// keeps the resource block minimal for users who already configure
// these values explicitly.
package otlp

import (
	"os"
	"path/filepath"
	"runtime"
)

// defaultAutoResource toggles WithAutoResource.
const defaultAutoResource = false

// applyAutoResource injects OTel semantic-convention attributes onto
// the SDK resource map. Called once during New().
func applyAutoResource(res map[string]string) {
	if _, ok := res["process.pid"]; !ok {
		res["process.pid"] = itoa(os.Getpid())
	}
	if _, ok := res["process.runtime.name"]; !ok {
		res["process.runtime.name"] = "go"
	}
	if _, ok := res["process.runtime.version"]; !ok {
		res["process.runtime.version"] = runtime.Version()
	}
	if _, ok := res["process.executable.name"]; !ok {
		res["process.executable.name"] = filepath.Base(os.Args[0])
	}
	if _, ok := res["host.arch"]; !ok {
		res["host.arch"] = runtime.GOARCH
	}
	if _, ok := res["os.type"]; !ok {
		res["os.type"] = runtime.GOOS
	}
}

// itoa is a small dependency-free alternative to strconv.Itoa for the
// handful of int -> string conversions the SDK needs.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
