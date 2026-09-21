package coupon_test

import (
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

func TestValidLength_Boundaries(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"1234567", false},    // 7 chars — below minimum
		{"12345678", true},    // 8 chars — minimum
		{"1234567890", true},  // 10 chars — maximum
		{"12345678901", false}, // 11 chars — above maximum
		{"", false},
	}
	for _, tc := range cases {
		if got := coupon.ValidLength(tc.code); got != tc.want {
			t.Errorf("ValidLength(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestUnavailableValidator_RejectsEverything(t *testing.T) {
	v := coupon.NewUnavailableValidator()
	for _, code := range []string{"HAPPYHRS", "FIFTYOFF", "ANYCODE1"} {
		if v.IsValid(code) {
			t.Errorf("unavailable validator should reject %q", code)
		}
	}
}
