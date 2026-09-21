// Package idgen generates RFC 4122 version-4 UUIDs using only the
// standard library — small enough not to justify a dependency.
package idgen

import (
	"crypto/rand"
	"fmt"
)

// NewUUID returns a random (version 4, variant 1) UUID string.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read only fails if the OS entropy source is
		// unavailable, which is unrecoverable for a process that
		// needs random ids at all.
		panic("idgen: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
