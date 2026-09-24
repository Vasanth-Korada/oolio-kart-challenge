package order

import (
	"context"
	"errors"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

// Errors returned by Service.PlaceOrder for invalid input. The HTTP layer
// maps each of them to 422.
var (
	ErrEmptyItems      = errors.New("order must contain at least one item")
	ErrInvalidQuantity = errors.New("item quantity must be greater than zero")
	ErrProductNotFound = errors.New("one or more products were not found")
	ErrInvalidCoupon   = errors.New("coupon code is invalid")
)

// CouponDiscountRate is applied to the order subtotal when a valid
// coupon is supplied.
const CouponDiscountRate = 0.05

// Item is one line of an order: a product and how many of it.
type Item struct {
	ProductID string
	Quantity  int
}

// Order is a placed order. Prices are computed server-side; Products holds
// each item's product as priced at order time.
type Order struct {
	ID         string
	Items      []Item
	Products   []product.Product
	CouponCode string
	Subtotal   float64
	Discount   float64
	Total      float64
}

// CreateOrderRequest is the input to Service.PlaceOrder.
type CreateOrderRequest struct {
	Items      []Item
	CouponCode string
}

// Repository stores placed orders.
type Repository interface {
	Create(ctx context.Context, o Order) (Order, error)
}

// Service is the order business logic, independent of HTTP and storage.
type Service interface {
	PlaceOrder(ctx context.Context, req CreateOrderRequest) (Order, error)
}
