package coupon_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
)

type LoadIndexSuite struct {
	suite.Suite
	dir string
}

func TestLoadIndexSuite(t *testing.T) {
	suite.Run(t, new(LoadIndexSuite))
}

func (s *LoadIndexSuite) SetupTest() {
	s.dir = s.T().TempDir()
}

func (s *LoadIndexSuite) TestMissingFileWrapsNotExist() {
	_, err := coupon.LoadIndex(filepath.Join(s.dir, "missing.idx"))

	// cmd/server relies on this to fall back to the unavailable validator.
	s.Require().ErrorIs(err, os.ErrNotExist)
}

func (s *LoadIndexSuite) TestRejectsBadFiles() {
	header := func(magic string, count byte) []byte {
		return []byte{magic[0], magic[1], magic[2], magic[3], 0, 0, 0, count}
	}
	tests := []struct {
		name string
		data []byte
	}{
		{name: "old CPX1 format", data: append(header("CPX1", 1), make([]byte, 16)...)},
		{name: "size does not match count", data: append(header("CPX2", 2), make([]byte, 11)...)},
		{name: "shorter than the header", data: []byte("CPX2")},
		{name: "empty file", data: []byte{}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			path := filepath.Join(s.dir, "bad.idx")
			s.Require().NoError(os.WriteFile(path, tt.data, 0o600))

			_, err := coupon.LoadIndex(path)

			s.Require().Error(err)
			s.NotErrorIs(err, os.ErrNotExist)
		})
	}
}
