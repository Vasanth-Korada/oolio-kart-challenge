package product

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBRepository struct {
	pool *pgxpool.Pool
}

func NewDBRepository(pool *pgxpool.Pool) *DBRepository {
	return &DBRepository{pool: pool}
}

func (r *DBRepository) List(ctx context.Context) ([]Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, price::float8, category FROM products ORDER BY id::int`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Category); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (r *DBRepository) GetByID(ctx context.Context, id string) (Product, error) {
	var p Product
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, price::float8, category FROM products WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Price, &p.Category)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, err
	}
	return p, nil
}

var _ Repository = (*DBRepository)(nil)
