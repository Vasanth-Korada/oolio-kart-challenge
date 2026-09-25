package order_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

type fakeCoupons map[string]bool

func (f fakeCoupons) IsValid(code string) bool { return f[code] }

// flatFive is the shipped default rule (5% of the subtotal) as a fake policy.
type flatFive struct{}

func (flatFive) Discount(_ string, subtotal float64) float64 { return subtotal * 0.05 }

// spyDiscounts records what the service asks the policy and returns amount.
type spyDiscounts struct {
	amount    float64
	codes     []string
	subtotals []float64
}

func (d *spyDiscounts) Discount(code string, subtotal float64) float64 {
	d.codes = append(d.codes, code)
	d.subtotals = append(d.subtotals, subtotal)
	return d.amount
}

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
	return order.NewService(products, coupons, flatFive{}, repo, nil, logger), repo
}

func (s *ServiceSuite) place(coupons fakeCoupons, couponCode string, items ...order.Item) (order.Order, error) {
	svc, _ := s.newService(coupons)
	return svc.PlaceOrder(context.Background(), order.CreateOrderRequest{Items: items, CouponCode: couponCode})
}

func item(productID string, quantity int) order.Item {
	return order.Item{ProductID: productID, Quantity: quantity}
}

// lines returns n copies of one line, e.g. to test the line-count cap.
func lines(n int, line order.Item) []order.Item {
	out := make([]order.Item, n)
	for i := range out {
		out[i] = line
	}
	return out
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
		{name: "lines at the maximum", items: lines(order.MaxLineItems, item("1", 1))},
		{name: "lines above the maximum", items: lines(order.MaxLineItems+1, item("1", 1)), wantErr: order.ErrTooManyItems},
		{name: "line cap is checked before quantities", items: lines(order.MaxLineItems+1, item("999", 0)), wantErr: order.ErrTooManyItems},
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

func (s *ServiceSuite) TestReportsEveryUnknownProduct() {
	tests := []struct {
		name  string
		items []order.Item
		want  string
	}{
		{name: "one unknown", items: []order.Item{item("1", 1), item("999", 1)}, want: ": product 999"},
		{name: "several unknown, in request order", items: []order.Item{item("999", 1), item("1", 1), item("11", 2)}, want: ": products 999, 11"},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			_, err := s.place(nil, "", tt.items...)

			s.Require().ErrorIs(err, order.ErrProductNotFound)
			s.True(strings.HasSuffix(err.Error(), tt.want), "error %q should end with %q", err, tt.want)
		})
	}
}

func (s *ServiceSuite) TestIgnoresClientSuppliedPrice() {
	got, err := s.place(nil, "", item("1", 1))

	s.Require().NoError(err)
	s.Equal(product.SeedProducts()[0].Price, got.Products[0].Price, "price must come from the seeded catalog")
}

// fakeRecorder captures the events PlaceOrder reports, in order.
type fakeRecorder struct{ events []string }

func (f *fakeRecorder) OrderPlaced(withCoupon bool, total float64) {
	f.events = append(f.events, fmt.Sprintf("placed coupon=%t total=%.2f", withCoupon, total))
}
func (f *fakeRecorder) OrderRejected(reason string) { f.events = append(f.events, "rejected "+reason) }
func (f *fakeRecorder) CouponChecked(valid bool) {
	f.events = append(f.events, fmt.Sprintf("coupon valid=%t", valid))
}

func (s *ServiceSuite) TestReportsEventsToRecorder() {
	tests := []struct {
		name       string
		items      []order.Item
		couponCode string
		want       []string
	}{
		{name: "order with a valid coupon", items: []order.Item{item("1", 1), item("9", 1)}, couponCode: "HAPPYHRS",
			want: []string{"coupon valid=true", "placed coupon=true total=9.50"}},
		{name: "order without a coupon checks none", items: []order.Item{item("1", 1)},
			want: []string{"placed coupon=false total=6.50"}},
		{name: "invalid coupon", items: []order.Item{item("1", 1)}, couponCode: "BADCODE1",
			want: []string{"coupon valid=false", "rejected invalid_coupon"}},
		{name: "empty items", want: []string{"rejected empty_items"}},
		{name: "too many lines", items: lines(order.MaxLineItems+1, item("1", 1)), want: []string{"rejected too_many_items"}},
		{name: "quantity too large", items: []order.Item{item("1", 1001)}, want: []string{"rejected quantity_too_large"}},
		{name: "unknown product", items: []order.Item{item("999", 1)}, want: []string{"rejected product_not_found"}},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			rec := &fakeRecorder{}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			products := product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
			svc := order.NewService(products, fakeCoupons{"HAPPYHRS": true}, flatFive{}, order.NewMemoryRepository(), rec, logger)

			_, _ = svc.PlaceOrder(context.Background(), order.CreateOrderRequest{Items: tt.items, CouponCode: tt.couponCode})

			s.Equal(tt.want, rec.events)
		})
	}
}

// spyProducts wraps the seeded catalog and counts GetMany calls, the only
// product call PlaceOrder makes (and, with Postgres, its only SELECT).
type spyProducts struct {
	product.Service
	getManyCalls int
}

func (p *spyProducts) GetMany(ctx context.Context, ids []string) (map[string]product.Product, error) {
	p.getManyCalls++
	return p.Service.GetMany(ctx, ids)
}

func (s *ServiceSuite) TestCouponIsCheckedBeforeProductLookup() {
	tests := []struct {
		name        string
		items       []order.Item
		couponCode  string
		wantErr     error
		wantLookups int
	}{
		{name: "invalid coupon never looks up products", items: []order.Item{item("1", 1)}, couponCode: "BADCODE1",
			wantErr: order.ErrInvalidCoupon, wantLookups: 0},
		{name: "invalid coupon wins over an unknown product", items: []order.Item{item("999", 1)}, couponCode: "BADCODE1",
			wantErr: order.ErrInvalidCoupon, wantLookups: 0},
		{name: "valid coupon then one lookup", items: []order.Item{item("1", 1)}, couponCode: "HAPPYHRS", wantLookups: 1},
		{name: "no coupon then one lookup", items: []order.Item{item("1", 1)}, wantLookups: 1},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			spy := &spyProducts{Service: product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)}
			svc := order.NewService(spy, fakeCoupons{"HAPPYHRS": true}, flatFive{}, order.NewMemoryRepository(), nil, logger)

			_, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{Items: tt.items, CouponCode: tt.couponCode})

			if tt.wantErr != nil {
				s.Require().ErrorIs(err, tt.wantErr)
			} else {
				s.Require().NoError(err)
			}
			s.Equal(tt.wantLookups, spy.getManyCalls)
		})
	}
}

func (s *ServiceSuite) TestAsksThePolicyForTheDiscount() {
	tests := []struct {
		name          string
		couponCode    string
		policyAmount  float64
		wantCodes     []string
		wantSubtotals []float64
		wantDiscount  float64
		wantTotal     float64
	}{
		// product "1" = $6.50, product "9" = $3.50 -> subtotal $10.00
		{name: "valid coupon: the policy prices it, the service rounds it", couponCode: "HAPPYHRS", policyAmount: 1.234,
			wantCodes: []string{"HAPPYHRS"}, wantSubtotals: []float64{10}, wantDiscount: 1.23, wantTotal: 8.77},
		{name: "no coupon: the policy is not asked", policyAmount: 1.234, wantDiscount: 0, wantTotal: 10},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			products := product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
			policy := &spyDiscounts{amount: tt.policyAmount}
			svc := order.NewService(products, fakeCoupons{"HAPPYHRS": true}, policy, order.NewMemoryRepository(), nil, logger)

			got, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{
				Items: []order.Item{item("1", 1), item("9", 1)}, CouponCode: tt.couponCode,
			})

			s.Require().NoError(err)
			s.Equal(tt.wantCodes, policy.codes)
			s.Equal(tt.wantSubtotals, policy.subtotals)
			s.Equal(tt.wantDiscount, got.Discount)
			s.Equal(tt.wantTotal, got.Total)
		})
	}
}
