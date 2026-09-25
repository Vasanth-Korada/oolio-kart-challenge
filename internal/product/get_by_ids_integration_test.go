//go:build integration

package product_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/postgres"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
	"github.com/Vasanth-Korada/oolio-kart-challenge/migrations"
)

// GetByIDsDBSuite checks the single-query batch lookup against Postgres.
type GetByIDsDBSuite struct {
	suite.Suite
	pool *pgxpool.Pool
	repo *product.DBRepository
}

func TestGetByIDsDBSuite(t *testing.T) {
	suite.Run(t, new(GetByIDsDBSuite))
}

func (s *GetByIDsDBSuite) SetupSuite() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		s.T().Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := postgres.Connect(ctx, dsn)
	s.Require().NoError(err)
	s.Require().NoError(postgres.Migrate(ctx, pool, migrations.FS))
	s.pool = pool
	s.repo = product.NewDBRepository(pool)
}

func (s *GetByIDsDBSuite) TearDownSuite() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *GetByIDsDBSuite) TestReturnsOnlyExistingProducts() {
	got, err := s.repo.GetByIDs(context.Background(), []string{"1", "10", "999"})

	s.Require().NoError(err)
	s.Len(got, 2)
	s.Equal("Waffle with Berries", got["1"].Name)
	s.Equal(9.0, got["10"].Price)
	s.NotEmpty(got["10"].Image.Thumbnail)
}

func (s *GetByIDsDBSuite) TestEmptyInput() {
	got, err := s.repo.GetByIDs(context.Background(), []string{})

	s.Require().NoError(err)
	s.Empty(got)
}
