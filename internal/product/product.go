package product

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("product not found")

type Product struct {
	ID       string
	Name     string
	Price    float64
	Category string
	Image    Image
}

// Image is part of the base OpenAPI spec; the assignment gave no real
// photography, so these are deterministic placeholders, not decoration.
type Image struct {
	Thumbnail string
	Mobile    string
	Tablet    string
	Desktop   string
}

type Repository interface {
	List(ctx context.Context) ([]Product, error)
	GetByID(ctx context.Context, id string) (Product, error)
}

type Service interface {
	List(ctx context.Context) ([]Product, error)
	Get(ctx context.Context, id string) (Product, error)
}
