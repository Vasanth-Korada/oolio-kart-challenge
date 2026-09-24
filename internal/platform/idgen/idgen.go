// Package idgen generates identifiers.
package idgen

import (
	"crypto/rand"
	"fmt"
)

// NewUUID returns a random (version 4) UUID from crypto/rand. It panics if
// the system's random source fails.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("idgen: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
