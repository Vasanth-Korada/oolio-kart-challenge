package coupon

import "sort"

// Index is the runtime Validator: the valid codes as sorted keys, loaded
// once from the file BuildIndex writes.
type Index struct {
	keys []codeKey
}

var _ Validator = (*Index)(nil)

func (idx *Index) IsValid(code string) bool {
	if !ValidLength(code) {
		return false
	}
	k := makeKey(code)
	i := sort.Search(len(idx.keys), func(i int) bool {
		return compareKeys(idx.keys[i], k) >= 0
	})
	return i < len(idx.keys) && idx.keys[i] == k
}

func (idx *Index) Len() int { return len(idx.keys) }
