//go:build integration

package order_test

import (
	"context"
	"os"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/idgen"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/postgres"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
	"github.com/Vasanth-Korada/oolio-kart-challenge/migrations"
)

func TestDBRepository_Create(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := order.NewDBRepository(pool)
	o := order.Order{
		ID:         idgen.NewUUID(),
		Items:      []order.Item{{ProductID: "1", Quantity: 2}},
		Products:   []product.Product{{ID: "1", Name: "Waffle with Berries", Price: 6.5, Category: "Waffle"}},
		CouponCode: "HAPPYHRS",
	}

	created, err := repo.Create(ctx, o)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != o.ID {
		t.Fatalf("ID = %q, want %q", created.ID, o.ID)
	}

	var (
		gotCoupon *string
		itemCount int
	)
	if err := pool.QueryRow(ctx, "SELECT coupon_code FROM orders WHERE id = $1", o.ID).Scan(&gotCoupon); err != nil {
		t.Fatalf("query orders: %v", err)
	}
	if gotCoupon == nil || *gotCoupon != "HAPPYHRS" {
		t.Fatalf("coupon_code = %v, want HAPPYHRS", gotCoupon)
	}

	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM order_items WHERE order_id = $1", o.ID).Scan(&itemCount); err != nil {
		t.Fatalf("query order_items: %v", err)
	}
	if itemCount != 1 {
		t.Fatalf("order_items count = %d, want 1", itemCount)
	}
}
