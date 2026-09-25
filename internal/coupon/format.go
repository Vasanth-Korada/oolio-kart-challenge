package coupon

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// Index file layout (big-endian):
//
//	bytes 0-3   magic "CPX2"
//	bytes 4-7   uint32 key count n
//	bytes 8-    n keys of keySize bytes each, sorted ascending
const (
	indexMagic  = "CPX2"
	headerBytes = 8
)

func writeIndex(w io.Writer, keys []codeKey) error {
	if len(keys) > math.MaxUint32 {
		return fmt.Errorf("coupon: %d valid codes exceeds the uint32 index header limit", len(keys))
	}

	buffered := bufio.NewWriter(w)
	var header [headerBytes]byte
	copy(header[0:4], indexMagic)
	binary.BigEndian.PutUint32(header[4:8], uint32(len(keys))) //nolint:gosec // bounds-checked above
	if _, err := buffered.Write(header[:]); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := buffered.Write(key[:]); err != nil {
			return err
		}
	}
	return buffered.Flush()
}

func parseIndex(data []byte) ([]codeKey, error) {
	if len(data) < headerBytes || string(data[0:4]) != indexMagic {
		return nil, errors.New("not a valid coupon index file")
	}
	count := binary.BigEndian.Uint32(data[4:8])
	want := headerBytes + int(count)*keySize
	if len(data) != want {
		return nil, fmt.Errorf("corrupt: size %d, expected %d for %d entries", len(data), want, count)
	}

	keys := make([]codeKey, count)
	for keyIndex := range keys {
		offset := headerBytes + keyIndex*keySize
		copy(keys[keyIndex][:], data[offset:offset+keySize])
	}
	return keys, nil
}

// LoadIndex reads and validates an index file written by BuildIndex.
// A missing file returns an error wrapping os.ErrNotExist.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is server config (COUPON_INDEX_PATH), not user input
	if err != nil {
		return nil, fmt.Errorf("coupon: read index %s: %w", path, err)
	}
	keys, err := parseIndex(data)
	if err != nil {
		return nil, fmt.Errorf("coupon: index %s: %w", path, err)
	}
	return &Index{keys: keys}, nil
}
