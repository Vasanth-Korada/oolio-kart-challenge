package order

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/idgen"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type service struct {
	products product.Service
	coupons  coupon.Validator
	repo     Repository
	logger   *slog.Logger
}

// NewService returns a Service that prices items with products, checks
// coupons with coupons, and stores orders in repo.
func NewService(products product.Service, coupons coupon.Validator, repo Repository, logger *slog.Logger) Service {
	return &service{products: products, coupons: coupons, repo: repo, logger: logger}
}

// PlaceOrder validates, merges and prices the request, applies the coupon
// discount, and stores the order. Invalid input returns one of the package's
// Err values.
func (s *service) PlaceOrder(ctx context.Context, req CreateOrderRequest) (Order, error) {
	if len(req.Items) == 0 {
		return Order{}, ErrEmptyItems
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

	resolved := make([]product.Product, 0, len(merged))
	subtotal := 0.0
	for _, item := range merged {
		p, err := s.products.Get(ctx, item.ProductID)
		if err != nil {
			if errors.Is(err, product.ErrNotFound) {
				return Order{}, fmt.Errorf("%w: product %s", ErrProductNotFound, item.ProductID)
			}
			return Order{}, err
		}
		resolved = append(resolved, p)
		subtotal += p.Price * float64(item.Quantity)
	}

	if req.CouponCode != "" && !s.coupons.IsValid(req.CouponCode) {
		s.logger.WarnContext(ctx, "order: rejected invalid coupon", slog.String("coupon_code", req.CouponCode))
		return Order{}, ErrInvalidCoupon
	}

	discount := 0.0
	if req.CouponCode != "" {
		discount = roundMoney(subtotal * CouponDiscountRate)
	}
	subtotal = roundMoney(subtotal)

	o := Order{
		ID:         idgen.NewUUID(),
		Items:      merged,
		Products:   resolved,
		CouponCode: req.CouponCode,
		Subtotal:   subtotal,
		Discount:   discount,
		Total:      roundMoney(subtotal - discount),
	}

	created, err := s.repo.Create(ctx, o)
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
	for i, id := range order {
		out[i] = Item{ProductID: id, Quantity: quantityByID[id]}
	}
	return out
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
