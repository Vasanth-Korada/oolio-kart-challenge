package order

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/idgen"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type service struct {
	products product.Service
	coupons  coupon.Validator
	repo     Repository
	rec      Recorder
	logger   *slog.Logger
}

// NewService returns a Service that prices items with products, checks
// coupons with coupons, stores orders in repo, and reports events to rec.
// A nil rec records nothing.
func NewService(products product.Service, coupons coupon.Validator, repo Repository, rec Recorder, logger *slog.Logger) Service {
	if rec == nil {
		rec = noopRecorder{}
	}
	return &service{products: products, coupons: coupons, repo: repo, rec: rec, logger: logger}
}

// PlaceOrder validates, merges and prices the request, applies the coupon
// discount, and stores the order. Invalid input returns one of the package's
// Err values. Each outcome is reported to the Recorder.
func (s *service) PlaceOrder(ctx context.Context, req CreateOrderRequest) (Order, error) {
	o, err := s.placeOrder(ctx, req)
	if err != nil {
		if reason, ok := rejectionReason(err); ok {
			s.rec.OrderRejected(reason)
		}
		return Order{}, err
	}
	s.rec.OrderPlaced(o.CouponCode != "", o.Total)
	return o, nil
}

// rejectionReason maps a validation error to a short metrics label; other
// errors (storage failures) are not rejections.
func rejectionReason(err error) (string, bool) {
	reasons := []struct {
		err    error
		reason string
	}{
		{ErrEmptyItems, "empty_items"},
		{ErrTooManyItems, "too_many_items"},
		{ErrInvalidQuantity, "invalid_quantity"},
		{ErrQuantityTooLarge, "quantity_too_large"},
		{ErrProductNotFound, "product_not_found"},
		{ErrInvalidCoupon, "invalid_coupon"},
	}
	for _, rule := range reasons {
		if errors.Is(err, rule.err) {
			return rule.reason, true
		}
	}
	return "", false
}

func (s *service) placeOrder(ctx context.Context, req CreateOrderRequest) (Order, error) {
	if len(req.Items) == 0 {
		return Order{}, ErrEmptyItems
	}
	if len(req.Items) > MaxLineItems {
		return Order{}, ErrTooManyItems
	}
	// Each raw quantity is bounded before merging, so summing duplicates
	// below can't overflow int.
	for _, item := range req.Items {
		if err := checkQuantity(item); err != nil {
			return Order{}, err
		}
	}

	// A client can send the same productId twice (double-click, a naive
	// integration, a retried partial request). order_items has a
	// composite (order_id, product_id) primary key, so inserting two
	// rows for the same product in one order would fail the transaction
	// with a constraint violation, not a clean validation error. Merging
	// by summing quantities is also just the more sensible interpretation
	// of "the same product appears twice in this cart."
	merged := mergeItems(req.Items)
	for _, item := range merged {
		if err := checkQuantity(item); err != nil {
			return Order{}, err
		}
	}

	// The coupon check is an in-memory binary search, so it runs before the
	// product lookup: an order with a bad coupon never reaches the database.
	valid := req.CouponCode == "" || s.coupons.IsValid(req.CouponCode)
	if req.CouponCode != "" {
		s.rec.CouponChecked(valid)
	}
	if !valid {
		s.logger.WarnContext(ctx, "order: rejected invalid coupon", slog.String("coupon_code", req.CouponCode))
		return Order{}, ErrInvalidCoupon
	}

	// One query for every product in the cart, instead of one per line: the
	// cost stays flat as the cart grows, and all prices come from the same
	// moment.
	ids := make([]string, len(merged))
	for lineIndex, item := range merged {
		ids[lineIndex] = item.ProductID
	}
	found, err := s.products.GetMany(ctx, ids)
	if err != nil {
		return Order{}, err
	}
	if missing := missingIDs(ids, found); len(missing) > 0 {
		return Order{}, fmt.Errorf("%w: %s", ErrProductNotFound, describeIDs(missing))
	}

	resolved := make([]product.Product, len(merged))
	subtotal := 0.0
	for lineIndex, item := range merged {
		resolved[lineIndex] = found[item.ProductID]
		subtotal += resolved[lineIndex].Price * float64(item.Quantity)
	}

	discount := 0.0
	if req.CouponCode != "" {
		discount = roundMoney(subtotal * CouponDiscountRate)
	}
	subtotal = roundMoney(subtotal)

	newOrder := Order{
		ID:         idgen.NewUUID(),
		Items:      merged,
		Products:   resolved,
		CouponCode: req.CouponCode,
		Subtotal:   subtotal,
		Discount:   discount,
		Total:      roundMoney(subtotal - discount),
	}

	created, err := s.repo.Create(ctx, newOrder)
	if err != nil {
		s.logger.ErrorContext(ctx, "order: create failed", slog.Any("error", err))
		return Order{}, err
	}

	s.logger.InfoContext(ctx, "order: created",
		slog.String("order_id", created.ID),
		slog.Int("item_count", len(created.Items)),
		slog.Float64("discount", created.Discount),
	)
	return created, nil
}

// missingIDs returns the ids with no product in found, in request order.
func missingIDs(ids []string, found map[string]product.Product) []string {
	var missing []string
	for _, id := range ids {
		if _, ok := found[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

// describeIDs renders "product 7" or "products 7, 11" for error messages.
func describeIDs(ids []string) string {
	if len(ids) == 1 {
		return "product " + ids[0]
	}
	return "products " + strings.Join(ids, ", ")
}

func checkQuantity(item Item) error {
	switch {
	case item.Quantity <= 0:
		return fmt.Errorf("%w: product %s", ErrInvalidQuantity, item.ProductID)
	case item.Quantity > MaxItemQuantity:
		return fmt.Errorf("%w: product %s", ErrQuantityTooLarge, item.ProductID)
	}
	return nil
}

// mergeItems sums quantities for repeated productIds, preserving each
// product's first-seen order so the response reads naturally.
func mergeItems(items []Item) []Item {
	quantityByID := make(map[string]int, len(items))
	order := make([]string, 0, len(items))
	for _, item := range items {
		if _, seen := quantityByID[item.ProductID]; !seen {
			order = append(order, item.ProductID)
		}
		quantityByID[item.ProductID] += item.Quantity
	}

	out := make([]Item, len(order))
	for index, id := range order {
		out[index] = Item{ProductID: id, Quantity: quantityByID[id]}
	}
	return out
}

func roundMoney(amount float64) float64 {
	return math.Round(amount*100) / 100
}
