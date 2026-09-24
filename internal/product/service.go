package product

import (
	"context"
	"log/slog"
)

type service struct {
	repo   Repository
	logger *slog.Logger
}

// NewService returns a Service that reads from repo.
func NewService(repo Repository, logger *slog.Logger) Service {
	return &service{repo: repo, logger: logger}
}

// List returns every product.
func (s *service) List(ctx context.Context) ([]Product, error) {
	products, err := s.repo.List(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "product: list failed", slog.Any("error", err))
		return nil, err
	}
	return products, nil
}

// Get returns the product with the given id, or ErrNotFound.
func (s *service) Get(ctx context.Context, id string) (Product, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.logger.WarnContext(ctx, "product: get failed", slog.String("product_id", id), slog.Any("error", err))
		return Product{}, err
	}
	return p, nil
}
