package coupon_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

const realIndexPath = "../../coupons/coupons.idx"

// RealIndexSuite checks the committed index against the codes documented
// in the README.
type RealIndexSuite struct {
	suite.Suite
	idx *coupon.Index
}

func TestRealIndexSuite(t *testing.T) {
	suite.Run(t, new(RealIndexSuite))
}

func (s *RealIndexSuite) SetupSuite() {
	if _, err := os.Stat(realIndexPath); err != nil {
		s.T().Skipf("coupons.idx not built yet (%v); run `make build-coupon-index`", err)
	}
	idx, err := coupon.LoadIndex(realIndexPath)
	s.Require().NoError(err)
	s.idx = idx
}

func (s *RealIndexSuite) TestHoldsEightValidCodes() {
	s.Equal(8, s.idx.Len())
}

func (s *RealIndexSuite) TestDocumentedExamples() {
	for code, valid := range map[string]bool{
		"HAPPYHRS": true,
		"FIFTYOFF": true,
		"SUPER100": false,
	} {
		s.Run(code, func() {
			s.Equal(valid, s.idx.IsValid(code))
		})
	}
}
