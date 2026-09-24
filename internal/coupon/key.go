package coupon

import "bytes"

// Codes are at most maxLength bytes, so each is stored as-is in a fixed
// array: exact matching, no hashing. The leading length byte keeps
// "ABCDEFGH" distinct from "ABCDEFGH\x00" despite the zero padding.
const keySize = 1 + maxLength

type codeKey [keySize]byte

func makeKey(code string) codeKey {
	var k codeKey
	k[0] = byte(len(code)) //nolint:gosec // callers check ValidLength first, so len is 8-10
	copy(k[1:], code)
	return k
}

func compareKeys(a, b codeKey) int {
	return bytes.Compare(a[:], b[:])
}
