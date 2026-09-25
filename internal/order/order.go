package order

import (
	"context"
	"errors"
	"fmt"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

// Errors returned by Service.PlaceOrder for invalid input. The HTTP layer
// maps each of them to 422.
var (
	ErrEmptyItems       = errors.New("order must contain at least one item")
	ErrTooManyItems     = fmt.Errorf("order can have at most %d items", MaxLineItems)
	ErrInvalidQuantity  = errors.New("item quantity must be greater than zero")
	ErrQuantityTooLarge = fmt.Errorf("item quantity must be at most %d", MaxItemQuantity)
	ErrProductNotFound  = errors.New("one or more products were not found")
	ErrInvalidCoupon    = errors.New("coupon code is invalid")
)

// MaxLineItems caps how many lines one order may send, before duplicates are
// merged. It is checked before any work that grows with the cart, so a
// request with a million lines costs one comparison, not a loop, a map and a
// huge query.
const MaxLineItems = 100

// MaxItemQuantity caps one line item, after duplicates are merged. It keeps
// order totals inside the database columns, so an absurd quantity is a 422,
// not a numeric overflow (500).
const MaxItemQuantity = 1000

// DiscountPolicy decides how much a valid coupon takes off the subtotal.
// The coupon.Validator decides whether a code is valid; the policy only
// prices it. The result must not exceed subtotal; the service rounds it.
type DiscountPolicy interface {
	Discount(code string, subtotal float64) float64
}

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

// Recorder receives order and coupon events, for metrics. Implementations
// must be safe for concurrent use.
type Recorder interface {
	OrderPlaced(withCoupon bool, total float64)
	OrderRejected(reason string)
	CouponChecked(valid bool)
}

type noopRecorder struct{}

func (noopRecorder) OrderPlaced(bool, float64) {}
func (noopRecorder) OrderRejected(string)      {}
func (noopRecorder) CouponChecked(bool)        {}

// Service is the order business logic, independent of HTTP and storage.
type Service interface {
	PlaceOrder(ctx context.Context, req CreateOrderRequest) (Order, error)
}
