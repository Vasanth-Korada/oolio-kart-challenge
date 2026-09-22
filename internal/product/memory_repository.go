package product

import "context"

type MemoryRepository struct {
	products []Product
	byID     map[string]Product
}

func NewMemoryRepository(seed []Product) *MemoryRepository {
	byID := make(map[string]Product, len(seed))
	for _, p := range seed {
		byID[p.ID] = p
	}
	return &MemoryRepository{products: seed, byID: byID}
}

func (r *MemoryRepository) List(_ context.Context) ([]Product, error) {
	out := make([]Product, len(r.products))
	copy(out, r.products)
	return out, nil
}

func (r *MemoryRepository) GetByID(_ context.Context, id string) (Product, error) {
	p, ok := r.byID[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return p, nil
}

var _ Repository = (*MemoryRepository)(nil)

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
	for i := range products {
		products[i].Image = seedImage(products[i].ID)
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
