// Package order holds the Order domain model and the interfaces
// (Repository, Service) the HTTP layer depends on. The order Service is
// the one place product pricing, coupon validation, and persistence come
// together, so it takes product.Service and coupon.Validator as
// collaborators rather than reaching into their storage directly.
package order

import (
	"context"
	"errors"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

// Sentinel errors the HTTP layer maps to specific status codes (see
// internal/httpapi/errors.go). Wrapped errors from deeper layers should
// use errors.Is against these, never string matching.
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
// Order schema: the requested items, plus the resolved Product record
// for each item (server-priced, never trusting client input).
//
// CouponCode is persisted for audit/analytics but intentionally left out
// of the OpenAPI Order response schema, so the HTTP layer never
// serializes it.
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
