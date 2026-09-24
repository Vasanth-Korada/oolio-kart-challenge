package coupon_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

type ValidatorSuite struct {
	suite.Suite
}

func TestValidatorSuite(t *testing.T) {
	suite.Run(t, new(ValidatorSuite))
}

func (s *ValidatorSuite) TestValidLength() {
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
		s.Run(tt.name, func() {
			s.Equal(tt.want, coupon.ValidLength(tt.code))
		})
	}
}

func (s *ValidatorSuite) TestUnavailableValidatorRejectsEverything() {
	v := coupon.NewUnavailableValidator()
	for _, code := range []string{"HAPPYHRS", "FIFTYOFF", "ANYCODE1"} {
		s.Run(code, func() {
			s.False(v.IsValid(code))
		})
	}
}
