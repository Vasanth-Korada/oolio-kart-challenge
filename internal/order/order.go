package order

import (
	"context"
	"errors"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

var (
	ErrEmptyItems      = errors.New("order must contain at least one item")
	ErrInvalidQuantity = errors.New("item quantity must be greater than zero")
	ErrProductNotFound = errors.New("one or more products were not found")
	ErrInvalidCoupon   = errors.New("coupon code is invalid")
)

type Item struct {
	ProductID string
	Quantity  int
}

// CouponCode is persisted but not part of the OpenAPI Order schema, so
// the HTTP layer never serializes it.
type Order struct {
	ID         string
	Items      []Item
	Products   []product.Product
	CouponCode string
}

type CreateOrderRequest struct {
	Items      []Item
	CouponCode string
}

type Repository interface {
	Create(ctx context.Context, o Order) (Order, error)
}

type Service interface {
	PlaceOrder(ctx context.Context, req CreateOrderRequest) (Order, error)
}
