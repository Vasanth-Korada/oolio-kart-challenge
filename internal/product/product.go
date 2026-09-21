// Package product holds the Product domain model and the
// Repository/Service interfaces the rest of the application depends on.
package product

import (
	"context"
	"errors"
)

// ErrNotFound is returned by a Repository or Service when no product
// matches the requested id.
var ErrNotFound = errors.New("product not found")

// Product is the domain model, matching the OpenAPI Product schema.
type Product struct {
	ID       string
	Name     string
	Price    float64
	Category string
}

// Repository is the storage seam for products. Implementations must
// return ErrNotFound (or an error wrapping it) when an id has no match.
type Repository interface {
	List(ctx context.Context) ([]Product, error)
	GetByID(ctx context.Context, id string) (Product, error)
}

// Service is the business-logic seam consumed by the HTTP layer.
type Service interface {
	List(ctx context.Context) ([]Product, error)
	Get(ctx context.Context, id string) (Product, error)
}
