// Package idgen generates identifiers.
package idgen

import (
	"crypto/rand"
	"fmt"
)

// NewUUID returns a random (version 4) UUID from crypto/rand. It panics if
// the system's random source fails.
func NewUUID() string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		panic("idgen: failed to read random bytes: " + err.Error())
	}
	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", random[0:4], random[4:6], random[6:8], random[8:10], random[10:16])
}
