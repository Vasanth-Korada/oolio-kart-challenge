package product_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
)

func newTestService() product.Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return product.NewService(product.NewMemoryRepository(product.SeedProducts()), logger)
}

func TestService_List(t *testing.T) {
	svc := newTestService()

	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(product.SeedProducts()) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(product.SeedProducts()))
	}
}

func TestService_Get(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{name: "known id", id: "1"},
		{name: "unknown id", id: "does-not-exist", wantErr: product.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.Get(context.Background(), tt.id)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got.ID != tt.id {
				t.Fatalf("ID = %q, want %q", got.ID, tt.id)
			}
		})
	}
}
