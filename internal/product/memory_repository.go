package product

import "context"

// MemoryRepository serves a fixed product list from memory. It is read-only,
// so it is safe for concurrent use.
type MemoryRepository struct {
	products []Product
	byID     map[string]Product
}

// NewMemoryRepository returns a MemoryRepository holding seed.
func NewMemoryRepository(seed []Product) *MemoryRepository {
	byID := make(map[string]Product, len(seed))
	for _, product := range seed {
		byID[product.ID] = product
	}
	return &MemoryRepository{products: seed, byID: byID}
}

// List returns a copy of every product, so callers can't modify the store.
func (r *MemoryRepository) List(_ context.Context) ([]Product, error) {
	out := make([]Product, len(r.products))
	copy(out, r.products)
	return out, nil
}

// GetByID returns the product with id, or ErrNotFound.
func (r *MemoryRepository) GetByID(_ context.Context, id string) (Product, error) {
	p, ok := r.byID[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return p, nil
}

// GetByIDs returns the products with the given ids, keyed by id.
func (r *MemoryRepository) GetByIDs(_ context.Context, ids []string) (map[string]Product, error) {
	products := make(map[string]Product, len(ids))
	for _, id := range ids {
		if p, ok := r.byID[id]; ok {
			products[id] = p
		}
	}
	return products, nil
}

var _ Repository = (*MemoryRepository)(nil)

// SeedProducts returns the catalog that migrations/0001 seeds into Postgres,
// for the in-memory store and tests.
func SeedProducts() []Product {
	products := []Product{
		{ID: "1", Name: "Waffle with Berries", Price: 6.50, Category: "Waffle"},
		{ID: "2", Name: "Vanilla Bean Crème Brûlée", Price: 7.00, Category: "Crème Brûlée"},
		{ID: "3", Name: "Macaron Mix of Five", Price: 8.00, Category: "Macaron"},
		{ID: "4", Name: "Classic Tiramisu", Price: 5.50, Category: "Tiramisu"},
		{ID: "5", Name: "Berry Basque Burnt Cheesecake", Price: 6.50, Category: "Cheesecake"},
		{ID: "6", Name: "Salted Caramel Macaron", Price: 8.00, Category: "Macaron"},
		{ID: "7", Name: "Chocolate Souffle", Price: 6.00, Category: "Souffle"},
		{ID: "8", Name: "Vanilla Panna Cotta", Price: 6.00, Category: "Panna Cotta"},
		{ID: "9", Name: "Oat & Raisin Cookie", Price: 3.50, Category: "Cookie"},
		{ID: "10", Name: "Chicken Waffle", Price: 9.00, Category: "Waffle"},
	}
	for index := range products {
		products[index].Image = seedImage(products[index].ID)
	}
	return products
}

// seedImage mirrors the URL pattern migrations/0004_add_product_images.sql
// backfills into Postgres, so the in-memory and DB-backed repositories
// agree on the same deterministic placeholder images.
func seedImage(id string) Image {
	seed := "product-" + id
	return Image{
		Thumbnail: "https://picsum.photos/seed/" + seed + "/150/150",
		Mobile:    "https://picsum.photos/seed/" + seed + "/375/250",
		Tablet:    "https://picsum.photos/seed/" + seed + "/600/400",
		Desktop:   "https://picsum.photos/seed/" + seed + "/900/600",
	}
}
