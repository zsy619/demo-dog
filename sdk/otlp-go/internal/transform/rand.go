// Hex random ID generation. crypto/rand is overkill but cheap; avoids
// pulling in google/uuid as a dependency.
package transform

import (
	"crypto/rand"
	"encoding/hex"
)

// Hex returns n random bytes encoded as hex. The fallback path is rare
// (only if /dev/urandom fails) and intentionally does not panic so SDK
// startup survives locked-down sandboxes.
func Hex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = 0
		}
	}
	return hex.EncodeToString(b)
}
