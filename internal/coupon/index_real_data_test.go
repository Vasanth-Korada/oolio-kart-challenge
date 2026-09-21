package coupon_test

import (
	"os"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

// Path to the committed, built index — skipped if absent (e.g. before
// `make build-coupon-index` has run) rather than failing CI.
const realIndexPath = "../../coupons/coupons.idx"

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
		t.Run(tc.code, func(t *testing.T) {
			if got := idx.IsValid(tc.code); got != tc.valid {
				t.Errorf("IsValid(%q) = %v, want %v", tc.code, got, tc.valid)
			}
		})
	}
}
