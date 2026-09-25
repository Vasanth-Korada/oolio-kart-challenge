package discount_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/discount"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
)

// Compile-time check: Policy satisfies the interface order consumes.
var _ order.DiscountPolicy = (*discount.Policy)(nil)

type PolicySuite struct {
	suite.Suite
}

func TestPolicySuite(t *testing.T) {
	suite.Run(t, new(PolicySuite))
}

func (s *PolicySuite) newPolicy(cfg discount.Config) *discount.Policy {
	policy, err := discount.New(cfg)
	s.Require().NoError(err)
	return policy
}

func (s *PolicySuite) TestDefaultMatchesTheOldFlatRate() {
	policy := s.newPolicy(discount.Config{DefaultPercent: 5})

	for _, subtotal := range []float64{0.01, 6.5, 10, 27.5, 45.5, 99.99, 1234.56, 9000} {
		s.Equal(subtotal*0.05, policy.Discount("ANYCODE1", subtotal), "subtotal %g", subtotal)
	}
}

func (s *PolicySuite) TestPerCodePercent() {
	policy := s.newPolicy(discount.Config{
		DefaultPercent: 5,
		Codes:          []discount.CodeRule{{Code: "TENPCTOFF", Percent: 10}, {Code: "ALLFREE1", Percent: 100}},
	})

	tests := []struct {
		name     string
		code     string
		subtotal float64
		want     float64
	}{
		{name: "code without a rule uses the default", code: "HAPPYHRS", subtotal: 100, want: 5},
		{name: "code with its own percent", code: "TENPCTOFF", subtotal: 100, want: 10},
		{name: "codes are case-sensitive", code: "tenpctoff", subtotal: 100, want: 5},
		{name: "100 percent equals the subtotal, never more", code: "ALLFREE1", subtotal: 42.5, want: 42.5},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.InDelta(tt.want, policy.Discount(tt.code, tt.subtotal), 1e-9)
		})
	}
	s.Equal(2, policy.Rules())
}

func (s *PolicySuite) TestInvalidConfig() {
	tests := []struct {
		name    string
		cfg     discount.Config
		wantErr string
	}{
		{name: "missing default", cfg: discount.Config{}, wantErr: "discount.defaultPercent: percent must be above 0 and at most 100, got 0"},
		{name: "default above 100", cfg: discount.Config{DefaultPercent: 101}, wantErr: "got 101"},
		{name: "negative default", cfg: discount.Config{DefaultPercent: -5}, wantErr: "got -5"},
		{name: "code percent out of range", cfg: discount.Config{DefaultPercent: 5,
			Codes: []discount.CodeRule{{Code: "HAPPYHRS", Percent: 0}}}, wantErr: `discount.codes["HAPPYHRS"]: percent must be above 0`},
		{name: "code of the wrong length", cfg: discount.Config{DefaultPercent: 5,
			Codes: []discount.CodeRule{{Code: "SHORT", Percent: 10}}}, wantErr: `discount.codes["SHORT"]: code must be 8 to 10 characters`},
		{name: "duplicate code", cfg: discount.Config{DefaultPercent: 5,
			Codes: []discount.CodeRule{{Code: "HAPPYHRS", Percent: 10}, {Code: "HAPPYHRS", Percent: 20}}}, wantErr: "listed more than once"},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, err := discount.New(tt.cfg)

			s.Require().Error(err)
			s.Contains(err.Error(), tt.wantErr)
		})
	}
}
