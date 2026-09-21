package order_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
)

func TestMemoryRepository_CreateAndAll(t *testing.T) {
	repo := order.NewMemoryRepository()

	o := order.Order{ID: "order-1", Items: []order.Item{{ProductID: "1", Quantity: 2}}}
	created, err := repo.Create(context.Background(), o)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != o.ID {
		t.Fatalf("created.ID = %q, want %q", created.ID, o.ID)
	}

	all := repo.All()
	if len(all) != 1 || all[0].ID != o.ID {
		t.Fatalf("All() = %+v, want one order with id %q", all, o.ID)
	}
}

func TestMemoryRepository_Concurrent(t *testing.T) {
	repo := order.NewMemoryRepository()
	const n = 50

	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func(i int) {
			_, _ = repo.Create(context.Background(), order.Order{ID: fmt.Sprintf("order-%d", i)})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}

	if got := len(repo.All()); got != n {
		t.Fatalf("len(All()) = %d, want %d", got, n)
	}
}
