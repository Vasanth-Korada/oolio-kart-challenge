package product_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

func TestMemoryRepository_List(t *testing.T) {
	seed := product.SeedProducts()
	repo := product.NewMemoryRepository(seed)

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(seed) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(seed))
	}
}

func TestMemoryRepository_List_ReturnsCopy(t *testing.T) {
	repo := product.NewMemoryRepository(product.SeedProducts())

	got, _ := repo.List(context.Background())
	got[0].Name = "mutated"

	got2, _ := repo.List(context.Background())
	if got2[0].Name == "mutated" {
		t.Fatal("List returned the internal slice, not a copy")
	}
}

func TestMemoryRepository_GetByID(t *testing.T) {
	repo := product.NewMemoryRepository(product.SeedProducts())

	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{name: "known id", id: "1"},
		{name: "unknown id", id: "nope", wantErr: product.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetByID(context.Background(), tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got.ID != tt.id {
				t.Fatalf("ID = %q, want %q", got.ID, tt.id)
			}
		})
	}
}
