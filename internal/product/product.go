package product

import (
	"context"
	"errors"
)

// ErrNotFound is returned when no product has the requested id.
var ErrNotFound = errors.New("product not found")

// Product is an item in the catalog.
type Product struct {
	ID       string
	Name     string
	Price    float64
	Category string
	Image    Image
}

// Image holds a product's image URLs for each screen size.
type Image struct {
	Thumbnail string
	Mobile    string
	Tablet    string
	Desktop   string
}

// Repository reads products from storage.
type Repository interface {
	List(ctx context.Context) ([]Product, error)
	GetByID(ctx context.Context, id string) (Product, error)
}

// Service is the catalog business logic, used by handlers and the order
// service.
type Service interface {
	List(ctx context.Context) ([]Product, error)
	Get(ctx context.Context, id string) (Product, error)
}
