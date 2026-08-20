package api

import "crypto/rand"

// cryptoRandBytes wraps crypto/rand.Read so trace context code can
// keep its import surface tight.
func cryptoRandBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand is documented to never fail in stdlib except
		// for catastrophic entropy source exhaustion, in which case
		// returning zero-bytes is the right degradation: the trace
		// context will still be syntactically valid and downstream
		// tooling will treat it as "unknown".
		return b
	}
	return b
}
