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

func NewService(products product.Service, coupons coupon.Validator, repo Repository, logger *slog.Logger) Service {
	return &service{products: products, coupons: coupons, repo: repo, logger: logger}
}

func (s *service) PlaceOrder(ctx context.Context, req CreateOrderRequest) (Order, error) {
	if len(req.Items) == 0 {
		return Order{}, ErrEmptyItems
	}

	resolved := make([]product.Product, 0, len(req.Items))
	subtotal := 0.0
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return Order{}, fmt.Errorf("%w: product %s", ErrInvalidQuantity, item.ProductID)
		}

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
		Items:      req.Items,
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

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}
