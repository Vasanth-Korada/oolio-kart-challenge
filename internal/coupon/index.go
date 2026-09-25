package coupon

import "sort"

// Index is the runtime Validator: the valid codes as sorted keys, loaded
// once from the file BuildIndex writes.
type Index struct {
	keys []codeKey
}

var _ Validator = (*Index)(nil)

// IsValid reports whether code is in the index. It does no I/O: a length
// check, then a binary search over the sorted keys.
func (idx *Index) IsValid(code string) bool {
	if !ValidLength(code) {
		return false
	}
	key := makeKey(code)
	position := sort.Search(len(idx.keys), func(candidate int) bool {
		return compareKeys(idx.keys[candidate], key) >= 0
	})
	return position < len(idx.keys) && idx.keys[position] == key
}

// Len returns the number of valid codes in the index.
func (idx *Index) Len() int { return len(idx.keys) }
