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

func scanProduct(row pgx.Row, p *Product) error {
	return row.Scan(&p.ID, &p.Name, &p.Price, &p.Category,
		&p.Image.Thumbnail, &p.Image.Mobile, &p.Image.Tablet, &p.Image.Desktop)
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
		var p Product
		if err := scanProduct(rows, &p); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

// GetByID returns the product with id, or ErrNotFound.
func (r *DBRepository) GetByID(ctx context.Context, id string) (Product, error) {
	var p Product
	row := r.pool.QueryRow(ctx, `SELECT `+productColumns+` FROM products WHERE id = $1`, id)
	err := scanProduct(row, &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, err
	}
	return p, nil
}

var _ Repository = (*DBRepository)(nil)
