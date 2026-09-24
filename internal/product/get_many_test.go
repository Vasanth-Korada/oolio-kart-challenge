package product_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

// GetManySuite covers the batch lookup through the service and the
// in-memory repository.
type GetManySuite struct {
	suite.Suite
	svc product.Service
}

func TestGetManySuite(t *testing.T) {
	suite.Run(t, new(GetManySuite))
}

func (s *GetManySuite) SetupTest() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s.svc = product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
}

func (s *GetManySuite) TestReturnsFoundProductsKeyedByID() {
	tests := []struct {
		name    string
		ids     []string
		wantIDs []string
	}{
		{name: "all found", ids: []string{"1", "3", "10"}, wantIDs: []string{"1", "3", "10"}},
		{name: "missing ids are absent, not an error", ids: []string{"1", "999", "11"}, wantIDs: []string{"1"}},
		{name: "nothing found", ids: []string{"999"}, wantIDs: []string{}},
		{name: "empty input", ids: []string{}, wantIDs: []string{}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			got, err := s.svc.GetMany(context.Background(), tt.ids)

			s.Require().NoError(err)
			s.Len(got, len(tt.wantIDs))
			for _, id := range tt.wantIDs {
				s.Equal(id, got[id].ID)
			}
		})
	}
}

func (s *GetManySuite) TestMatchesGet() {
	got, err := s.svc.GetMany(context.Background(), []string{"2"})
	s.Require().NoError(err)
	one, err := s.svc.Get(context.Background(), "2")
	s.Require().NoError(err)

	s.Equal(one, got["2"])
}
