package coupon_test

import (
	"os"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

// realIndexPath points at the committed, built index relative to this
// package. This test is skipped if it isn't present (e.g. a fresh clone
// before `make build-coupon-index` has run) rather than failing CI.
const realIndexPath = "../../coupons/coupons.idx"

// TestRealIndex_DocumentedExamples checks the exact examples from
// Oolio's assignment README against the index built from the real
// couponbase1/2/3.gz files — not a synthetic fixture, the actual data.
func TestRealIndex_DocumentedExamples(t *testing.T) {
	if _, err := os.Stat(realIndexPath); err != nil {
		t.Skipf("coupons.idx not built yet (%v); run `make build-coupon-index`", err)
	}

	idx, err := coupon.LoadIndex(realIndexPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	cases := []struct {
		code  string
		valid bool
	}{
		{"HAPPYHRS", true},
		{"FIFTYOFF", true},
		{"SUPER100", false},
	}
	for _, tc := range cases {
		if got := idx.IsValid(tc.code); got != tc.valid {
			t.Errorf("IsValid(%q) = %v, want %v", tc.code, got, tc.valid)
		}
	}
}
