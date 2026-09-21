package order

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu     sync.Mutex
	orders map[string]Order
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{orders: make(map[string]Order)}
}

func (r *MemoryRepository) Create(_ context.Context, o Order) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[o.ID] = o
	return o, nil
}

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
