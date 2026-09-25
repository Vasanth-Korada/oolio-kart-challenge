package product_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

// ServiceSuite covers the product service over the seeded in-memory
// repository: List, Get and the batch lookup GetMany.
type ServiceSuite struct {
	suite.Suite
	svc product.Service
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceSuite))
}

func (s *ServiceSuite) SetupTest() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s.svc = product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
}

func (s *ServiceSuite) TestList() {
	got, err := s.svc.List(context.Background())

	s.Require().NoError(err)
	s.Len(got, len(product.SeedProducts()))
}

func (s *ServiceSuite) TestGet() {
	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{name: "known id", id: "1"},
		{name: "unknown id", id: "does-not-exist", wantErr: product.ErrNotFound},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			got, err := s.svc.Get(context.Background(), tt.id)

			if tt.wantErr != nil {
				s.Require().ErrorIs(err, tt.wantErr)
				return
			}
			s.Require().NoError(err)
			s.Equal(tt.id, got.ID)
		})
	}
}

func (s *ServiceSuite) TestGetManyReturnsFoundProductsKeyedByID() {
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

func (s *ServiceSuite) TestGetManyMatchesGet() {
	got, err := s.svc.GetMany(context.Background(), []string{"2"})
	s.Require().NoError(err)
	one, err := s.svc.Get(context.Background(), "2")
	s.Require().NoError(err)

	s.Equal(one, got["2"])
}
