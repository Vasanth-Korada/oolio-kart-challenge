package product

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBRepository reads products from Postgres.
type DBRepository struct {
	pool *pgxpool.Pool
}

// NewDBRepository returns a DBRepository backed by pool.
func NewDBRepository(pool *pgxpool.Pool) *DBRepository {
	return &DBRepository{pool: pool}
}

const productColumns = `id, name, price::float8, category,
	COALESCE(image_thumbnail, ''), COALESCE(image_mobile, ''), COALESCE(image_tablet, ''), COALESCE(image_desktop, '')`

func scanProduct(row pgx.Row, product *Product) error {
	return row.Scan(&product.ID, &product.Name, &product.Price, &product.Category,
		&product.Image.Thumbnail, &product.Image.Mobile, &product.Image.Tablet, &product.Image.Desktop)
}

// List returns every product, ordered by numeric id.
func (r *DBRepository) List(ctx context.Context) ([]Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+productColumns+` FROM products ORDER BY id::int`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		if err := scanProduct(rows, &product); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

// GetByID returns the product with id, or ErrNotFound.
func (r *DBRepository) GetByID(ctx context.Context, id string) (Product, error) {
	var product Product
	row := r.pool.QueryRow(ctx, `SELECT `+productColumns+` FROM products WHERE id = $1`, id)
	err := scanProduct(row, &product)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, err
	}
	return product, nil
}

// GetByIDs returns the products with the given ids in a single query
// (WHERE id = ANY($1)), keyed by id.
func (r *DBRepository) GetByIDs(ctx context.Context, ids []string) (map[string]Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+productColumns+` FROM products WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make(map[string]Product, len(ids))
	for rows.Next() {
		var product Product
		if err := scanProduct(rows, &product); err != nil {
			return nil, err
		}
		products[product.ID] = product
	}
	return products, rows.Err()
}

var _ Repository = (*DBRepository)(nil)
