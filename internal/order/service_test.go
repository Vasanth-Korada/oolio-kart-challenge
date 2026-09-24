package order_test

import (
	"context"
	"io"
	"log/slog"
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type fakeCoupons map[string]bool

func (f fakeCoupons) IsValid(code string) bool { return f[code] }

type ServiceSuite struct {
	suite.Suite
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceSuite))
}

func (s *ServiceSuite) newService(coupons fakeCoupons) (order.Service, *order.MemoryRepository) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	products := product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
	repo := order.NewMemoryRepository()
	return order.NewService(products, coupons, repo, logger), repo
}

func (s *ServiceSuite) place(coupons fakeCoupons, couponCode string, items ...order.Item) (order.Order, error) {
	svc, _ := s.newService(coupons)
	return svc.PlaceOrder(context.Background(), order.CreateOrderRequest{Items: items, CouponCode: couponCode})
}

func item(productID string, quantity int) order.Item {
	return order.Item{ProductID: productID, Quantity: quantity}
}

func (s *ServiceSuite) TestValidation() {
	tests := []struct {
		name       string
		items      []order.Item
		couponCode string
		coupons    fakeCoupons
		wantErr    error
	}{
		{name: "empty items", wantErr: order.ErrEmptyItems},
		{name: "zero quantity", items: []order.Item{item("1", 0)}, wantErr: order.ErrInvalidQuantity},
		{name: "negative quantity", items: []order.Item{item("1", -1)}, wantErr: order.ErrInvalidQuantity},
		{name: "quantity at the maximum", items: []order.Item{item("1", order.MaxItemQuantity)}},
		{name: "quantity above the maximum", items: []order.Item{item("1", order.MaxItemQuantity+1)}, wantErr: order.ErrQuantityTooLarge},
		{name: "quantity that overflowed Postgres", items: []order.Item{item("10", 20_000_000)}, wantErr: order.ErrQuantityTooLarge},
		{name: "duplicates merged above the maximum", items: []order.Item{item("1", 600), item("1", 600)}, wantErr: order.ErrQuantityTooLarge},
		{name: "duplicates that would overflow int when merged", items: []order.Item{item("1", math.MaxInt), item("1", math.MaxInt)}, wantErr: order.ErrQuantityTooLarge},
		{name: "unknown product", items: []order.Item{item("999", 1)}, wantErr: order.ErrProductNotFound},
		{name: "invalid coupon", items: []order.Item{item("1", 1)}, couponCode: "BADCODE1", coupons: fakeCoupons{}, wantErr: order.ErrInvalidCoupon},
		{name: "valid order, no coupon", items: []order.Item{item("1", 2)}},
		{name: "valid order, valid coupon", items: []order.Item{item("1", 1)}, couponCode: "HAPPYHRS", coupons: fakeCoupons{"HAPPYHRS": true}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, err := s.place(tt.coupons, tt.couponCode, tt.items...)

			if tt.wantErr == nil {
				s.Require().NoError(err)
				return
			}
			s.Require().ErrorIs(err, tt.wantErr)
		})
	}
}

func (s *ServiceSuite) TestPricesAndPersistence() {
	svc, repo := s.newService(fakeCoupons{"HAPPYHRS": true})

	got, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{
		Items:      []order.Item{item("1", 2), item("3", 1)},
		CouponCode: "HAPPYHRS",
	})

	s.Require().NoError(err)
	s.NotEmpty(got.ID, "expected a generated order id")
	s.Len(got.Products, 2)
	s.Equal("HAPPYHRS", got.CouponCode)
	persisted := repo.All()
	s.Require().Len(persisted, 1)
	s.Equal(got.ID, persisted[0].ID)
}

func (s *ServiceSuite) TestDiscount() {
	// product "1" = $6.50, product "9" = $3.50 -> subtotal $10.00
	tests := []struct {
		name         string
		couponCode   string
		coupons      fakeCoupons
		wantSubtotal float64
		wantDiscount float64
		wantTotal    float64
	}{
		{name: "no coupon", wantSubtotal: 10.00, wantDiscount: 0, wantTotal: 10.00},
		{name: "valid coupon, flat 5% off", couponCode: "HAPPYHRS", coupons: fakeCoupons{"HAPPYHRS": true}, wantSubtotal: 10.00, wantDiscount: 0.50, wantTotal: 9.50},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			got, err := s.place(tt.coupons, tt.couponCode, item("1", 1), item("9", 1))

			s.Require().NoError(err)
			s.Equal(tt.wantSubtotal, got.Subtotal)
			s.Equal(tt.wantDiscount, got.Discount)
			s.Equal(tt.wantTotal, got.Total)
		})
	}
}

func (s *ServiceSuite) TestMergesDuplicateProductIDs() {
	svc, repo := s.newService(nil)

	got, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{
		Items: []order.Item{item("1", 1), item("1", 2)},
	})

	s.Require().NoError(err)
	s.Require().Len(got.Items, 1, "duplicates should merge into one line item")
	s.Equal(3, got.Items[0].Quantity)
	s.Equal(product.SeedProducts()[0].Price*3, got.Subtotal)
	persisted := repo.All()
	s.Require().Len(persisted, 1)
	s.Len(persisted[0].Items, 1)
}

func (s *ServiceSuite) TestIgnoresClientSuppliedPrice() {
	got, err := s.place(nil, "", item("1", 1))

	s.Require().NoError(err)
	s.Equal(product.SeedProducts()[0].Price, got.Products[0].Price, "price must come from the seeded catalog")
}
