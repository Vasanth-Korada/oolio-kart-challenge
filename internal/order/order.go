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

// CouponDiscountRate is applied to the order subtotal when a valid
// coupon is supplied. The spec never defines a per-code discount
// amount, so this is a flat rate across every valid code.
const CouponDiscountRate = 0.05

type Item struct {
	ProductID string
	Quantity  int
}

// Subtotal/Discount/Total and CouponCode are beyond the OpenAPI Order
// schema (which has no pricing fields) — a documented extension so a
// valid coupon has a visible effect, not just a pass/fail gate.
type Order struct {
	ID         string
	Items      []Item
	Products   []product.Product
	CouponCode string
	Subtotal   float64
	Discount   float64
	Total      float64
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
