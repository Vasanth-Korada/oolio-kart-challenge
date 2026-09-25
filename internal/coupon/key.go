package coupon

import "bytes"

// Codes are at most maxLength bytes, so each is stored as-is in a fixed
// array: exact matching, no hashing. The leading length byte keeps
// "ABCDEFGH" distinct from "ABCDEFGH\x00" despite the zero padding.
const keySize = 1 + maxLength

type codeKey [keySize]byte

func makeKey(code string) codeKey {
	var key codeKey
	key[0] = byte(len(code)) //nolint:gosec // callers check ValidLength first, so len is 8-10
	copy(key[1:], code)
	return key
}

func compareKeys(left, right codeKey) int {
	return bytes.Compare(left[:], right[:])
}
