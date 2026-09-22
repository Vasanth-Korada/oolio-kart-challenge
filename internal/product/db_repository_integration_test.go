//go:build integration

package product_test

import (
	"context"
	"os"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/postgres"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
	"github.com/Vasanth-Korada/oolio-kart-challenge/migrations"
)

func TestDBRepository(t *testing.T) {
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

	repo := product.NewDBRepository(pool)

	t.Run("List", func(t *testing.T) {
		got, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) == 0 {
			t.Fatal("expected the seeded catalog to be non-empty")
		}
	})

	t.Run("GetByID known id", func(t *testing.T) {
		got, err := repo.GetByID(ctx, "1")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.ID != "1" {
			t.Fatalf("ID = %q, want 1", got.ID)
		}
	})

	t.Run("GetByID unknown id", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, "does-not-exist"); err != product.ErrNotFound {
			t.Fatalf("err = %v, want %v", err, product.ErrNotFound)
		}
	})
}
