package coupon_test

import (
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

func TestValidLength(t *testing.T) {
	tests := []struct {
		name string
		code string
		want bool
	}{
		{name: "7 chars, below minimum", code: "1234567", want: false},
		{name: "8 chars, minimum", code: "12345678", want: true},
		{name: "10 chars, maximum", code: "1234567890", want: true},
		{name: "11 chars, above maximum", code: "12345678901", want: false},
		{name: "empty", code: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coupon.ValidLength(tt.code); got != tt.want {
				t.Errorf("ValidLength(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestUnavailableValidator(t *testing.T) {
	v := coupon.NewUnavailableValidator()
	for _, code := range []string{"HAPPYHRS", "FIFTYOFF", "ANYCODE1"} {
		t.Run(code, func(t *testing.T) {
			if v.IsValid(code) {
				t.Errorf("IsValid(%q) = true, want false", code)
			}
		})
	}
}
