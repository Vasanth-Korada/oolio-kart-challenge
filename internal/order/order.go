// Package order holds the Order domain model and the Repository/Service
// interfaces the HTTP layer depends on.
package order

import (
	"context"
	"errors"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

// Sentinel errors the HTTP layer maps to specific status codes.
var (
	ErrEmptyItems      = errors.New("order must contain at least one item")
	ErrInvalidQuantity = errors.New("item quantity must be greater than zero")
	ErrProductNotFound = errors.New("one or more products were not found")
	ErrInvalidCoupon   = errors.New("coupon code is invalid")
)

// Item is a line item on an order, matching the OpenAPI OrderReq/Order
// item shape (productId, quantity).
type Item struct {
	ProductID string
	Quantity  int
}

// Order is the domain model returned to callers, matching the OpenAPI
// Order schema plus CouponCode, which is persisted but intentionally
// left out of the JSON response (not part of that schema).
type Order struct {
	ID         string
	Items      []Item
	Products   []product.Product
	CouponCode string
}

// CreateOrderRequest is the input to Service.PlaceOrder, matching the
// OpenAPI OrderReq schema. CouponCode is optional; empty means none.
type CreateOrderRequest struct {
	Items      []Item
	CouponCode string
}

// Repository is the storage seam for orders.
type Repository interface {
	Create(ctx context.Context, o Order) (Order, error)
}

// Service is the business-logic seam consumed by the HTTP layer.
type Service interface {
	PlaceOrder(ctx context.Context, req CreateOrderRequest) (Order, error)
}
