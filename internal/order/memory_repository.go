package order

import (
	"context"
	"sync"
)

// MemoryRepository is an in-process Repository used by tests so they
// don't need a real Postgres instance to exercise order creation.
type MemoryRepository struct {
	mu     sync.Mutex
	orders map[string]Order
}

// NewMemoryRepository builds an empty MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{orders: make(map[string]Order)}
}

func (r *MemoryRepository) Create(_ context.Context, o Order) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = o
	return o, nil
}

// All returns every order created so far, for test assertions.
func (r *MemoryRepository) All() []Order {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Order, 0, len(r.orders))
	for _, o := range r.orders {
		out = append(out, o)
	}
	return out
}

var _ Repository = (*MemoryRepository)(nil)
