package order_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type fakeCoupons map[string]bool

func (f fakeCoupons) IsValid(code string) bool { return f[code] }

func newTestService(coupons fakeCoupons) (order.Service, *order.MemoryRepository) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	products := product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
	repo := order.NewMemoryRepository()
	return order.NewService(products, coupons, repo, logger), repo
}

func TestPlaceOrder(t *testing.T) {
	tests := []struct {
		name       string
		items      []order.Item
		couponCode string
		coupons    fakeCoupons
		wantErr    error
	}{
		{name: "empty items", items: nil, wantErr: order.ErrEmptyItems},
		{name: "zero quantity", items: []order.Item{{ProductID: "1", Quantity: 0}}, wantErr: order.ErrInvalidQuantity},
		{name: "negative quantity", items: []order.Item{{ProductID: "1", Quantity: -1}}, wantErr: order.ErrInvalidQuantity},
		{name: "unknown product", items: []order.Item{{ProductID: "999", Quantity: 1}}, wantErr: order.ErrProductNotFound},
		{
			name:       "invalid coupon",
			items:      []order.Item{{ProductID: "1", Quantity: 1}},
			couponCode: "BADCODE1",
			coupons:    fakeCoupons{},
			wantErr:    order.ErrInvalidCoupon,
		},
		{name: "valid order, no coupon", items: []order.Item{{ProductID: "1", Quantity: 2}}},
		{
			name:       "valid order, valid coupon",
			items:      []order.Item{{ProductID: "1", Quantity: 1}},
			couponCode: "HAPPYHRS",
			coupons:    fakeCoupons{"HAPPYHRS": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService(tt.coupons)
			_, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{
				Items:      tt.items,
				CouponCode: tt.couponCode,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlaceOrder_PricesAndPersistence(t *testing.T) {
	svc, repo := newTestService(fakeCoupons{"HAPPYHRS": true})

	got, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{
		Items:      []order.Item{{ProductID: "1", Quantity: 2}, {ProductID: "3", Quantity: 1}},
		CouponCode: "HAPPYHRS",
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	if got.ID == "" {
		t.Fatal("expected a generated order id")
	}
	if len(got.Products) != 2 {
		t.Fatalf("len(Products) = %d, want 2", len(got.Products))
	}
	if got.CouponCode != "HAPPYHRS" {
		t.Fatalf("CouponCode = %q, want HAPPYHRS", got.CouponCode)
	}

	persisted := repo.All()
	if len(persisted) != 1 || persisted[0].ID != got.ID {
		t.Fatalf("order not persisted correctly: %+v", persisted)
	}
}

// TestPlaceOrder_IgnoresClientSuppliedPrice guards against a client
// ever being able to influence what it's charged: Item carries no price
// field at all, so this is really testing that the resolved Product
// (and its price) always comes from the product service.
func TestPlaceOrder_IgnoresClientSuppliedPrice(t *testing.T) {
	svc, _ := newTestService(nil)

	got, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{
		Items: []order.Item{{ProductID: "1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	want := product.SeedProducts()[0].Price
	if got.Products[0].Price != want {
		t.Fatalf("price = %v, want %v (from the seeded catalog)", got.Products[0].Price, want)
	}
}
