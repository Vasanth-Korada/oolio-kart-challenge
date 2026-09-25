package order

import (
	"context"
	"sync"
)

// MemoryRepository keeps orders in memory. It is safe for concurrent use;
// data is lost on restart.
type MemoryRepository struct {
	mu     sync.Mutex
	orders map[string]Order
}

// NewMemoryRepository returns an empty MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{orders: make(map[string]Order)}
}

// Create stores the order.
func (r *MemoryRepository) Create(_ context.Context, order Order) (Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.ID] = order
	return order, nil
}

// All returns every stored order, in no particular order.
func (r *MemoryRepository) All() []Order {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Order, 0, len(r.orders))
	for _, order := range r.orders {
		out = append(out, order)
	}
	return out
}

var _ Repository = (*MemoryRepository)(nil)
