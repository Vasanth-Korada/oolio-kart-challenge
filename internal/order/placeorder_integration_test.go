//go:build integration

package order_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/postgres"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
	"github.com/Vasanth-Korada/oolio-kart-challenge/migrations"
)

// statementCounter is a pgx tracer that counts every statement sent to
// Postgres, including BEGIN and COMMIT: each one is a network round trip.
type statementCounter struct{ n atomic.Int64 }

func (c *statementCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *statementCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// batchedStatements is what one order costs when lookups and inserts are
// batched: SELECT products, BEGIN, INSERT orders, INSERT order_items, COMMIT.
const batchedStatements = 5

// PlaceOrderDBSuite runs PlaceOrder against a real Postgres and counts the
// statements it sends, to prove the cost doesn't grow with the cart size.
type PlaceOrderDBSuite struct {
	suite.Suite
	pool    *pgxpool.Pool
	counter *statementCounter
	svc     order.Service
}

func TestPlaceOrderDBSuite(t *testing.T) {
	suite.Run(t, new(PlaceOrderDBSuite))
}

func (s *PlaceOrderDBSuite) SetupSuite() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		s.T().Skip("DATABASE_URL not set")
	}
	ctx := context.Background()

	migratePool, err := postgres.Connect(ctx, dsn)
	s.Require().NoError(err)
	s.Require().NoError(postgres.Migrate(ctx, migratePool, migrations.FS))
	migratePool.Close()

	cfg, err := pgxpool.ParseConfig(dsn)
	s.Require().NoError(err)
	s.counter = &statementCounter{}
	cfg.ConnConfig.Tracer = s.counter
	s.pool, err = pgxpool.NewWithConfig(ctx, cfg)
	s.Require().NoError(err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	products := product.NewService(product.NewDBRepository(s.pool), logger)
	s.svc = order.NewService(products, nil, flatFive{}, order.NewDBRepository(s.pool), nil, logger)
}

func (s *PlaceOrderDBSuite) TearDownSuite() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// cart returns one line for each of the first n catalog products (ids 1..n).
func cart(n int) []order.Item {
	items := make([]order.Item, n)
	for i := range items {
		items[i] = order.Item{ProductID: strconv.Itoa(i + 1), Quantity: 1}
	}
	return items
}

func (s *PlaceOrderDBSuite) TestStatementsPerOrderDoNotGrowWithCartSize() {
	for _, size := range []int{1, 5, 10} {
		s.Run(strconv.Itoa(size)+" products", func() {
			s.counter.n.Store(0)

			placed, err := s.svc.PlaceOrder(context.Background(), order.CreateOrderRequest{Items: cart(size)})

			s.Require().NoError(err)
			s.Len(placed.Items, size)
			s.T().Logf("%d products → %d statements", size, s.counter.n.Load())
			s.Equal(int64(batchedStatements), s.counter.n.Load())
		})
	}
}

func (s *PlaceOrderDBSuite) TestEveryLineIsStoredWithItsSnapshotPrice() {
	ctx := context.Background()
	placed, err := s.svc.PlaceOrder(ctx, order.CreateOrderRequest{Items: []order.Item{
		{ProductID: "1", Quantity: 3}, {ProductID: "3", Quantity: 1}, {ProductID: "10", Quantity: 2},
	}})
	s.Require().NoError(err)

	rows, err := s.pool.Query(ctx,
		`SELECT product_id, quantity, unit_price::float8 FROM order_items WHERE order_id = $1 ORDER BY product_id::int`, placed.ID)
	s.Require().NoError(err)
	type line struct {
		id    string
		qty   int
		price float64
	}
	got, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (line, error) {
		var l line
		return l, r.Scan(&l.id, &l.qty, &l.price)
	})
	s.Require().NoError(err)
	s.Equal([]line{{"1", 3, 6.5}, {"3", 1, 8}, {"10", 2, 9}}, got)
	s.Equal(45.5, placed.Subtotal, "6.50×3 + 8.00×1 + 9.00×2")
}

func (s *PlaceOrderDBSuite) TestUnknownProductsAreAllReportedAndNothingIsStored() {
	ctx := context.Background()
	var before int
	s.Require().NoError(s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&before))

	_, err := s.svc.PlaceOrder(ctx, order.CreateOrderRequest{Items: []order.Item{
		{ProductID: "1", Quantity: 1}, {ProductID: "999", Quantity: 1}, {ProductID: "11", Quantity: 1},
	}})

	s.Require().ErrorIs(err, order.ErrProductNotFound)
	s.Contains(err.Error(), "999")
	s.Contains(err.Error(), "11")
	var after int
	s.Require().NoError(s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&after))
	s.Equal(before, after)
}

func (s *PlaceOrderDBSuite) TestInvalidCouponSendsNoStatements() {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	products := product.NewService(product.NewDBRepository(s.pool), logger)
	svc := order.NewService(products, fakeCoupons{"HAPPYHRS": true}, flatFive{}, order.NewDBRepository(s.pool), nil, logger)

	tests := []struct {
		name           string
		couponCode     string
		wantErr        error
		wantStatements int64
	}{
		{name: "invalid coupon: rejected before the database", couponCode: "BADCODE1", wantErr: order.ErrInvalidCoupon, wantStatements: 0},
		{name: "valid coupon: the usual batched order", couponCode: "HAPPYHRS", wantStatements: batchedStatements},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.counter.n.Store(0)

			_, err := svc.PlaceOrder(context.Background(), order.CreateOrderRequest{Items: cart(3), CouponCode: tt.couponCode})

			if tt.wantErr != nil {
				s.Require().ErrorIs(err, tt.wantErr)
			} else {
				s.Require().NoError(err)
			}
			s.Equal(tt.wantStatements, s.counter.n.Load())
		})
	}
}
