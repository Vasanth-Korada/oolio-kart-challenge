package order

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository is the production Repository implementation. It
// writes the order header and its line items in a single transaction so
// a partial order can never be observed.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a Repository backed by Postgres.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, o Order) (Order, error) {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// Stored as NULL, not "", when no coupon was applied.
		var couponCode *string
		if o.CouponCode != "" {
			couponCode = &o.CouponCode
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO orders (id, coupon_code) VALUES ($1, $2)`,
			o.ID, couponCode,
		); err != nil {
			return err
		}

		priceByProductID := make(map[string]float64, len(o.Products))
		for _, p := range o.Products {
			priceByProductID[p.ID] = p.Price
		}

		for _, item := range o.Items {
			if _, err := tx.Exec(ctx,
				`INSERT INTO order_items (order_id, product_id, quantity, unit_price)
				 VALUES ($1, $2, $3, $4)`,
				o.ID, item.ProductID, item.Quantity, priceByProductID[item.ProductID],
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Order{}, err
	}
	return o, nil
}

// compile-time check that PostgresRepository satisfies Repository.
var _ Repository = (*PostgresRepository)(nil)
