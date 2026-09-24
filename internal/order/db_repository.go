package order

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBRepository stores orders in Postgres.
type DBRepository struct {
	pool *pgxpool.Pool
}

// NewDBRepository returns a DBRepository backed by pool.
func NewDBRepository(pool *pgxpool.Pool) *DBRepository {
	return &DBRepository{pool: pool}
}

// Create inserts the order and its items in one transaction, storing each
// item's unit price as it was at order time.
func (r *DBRepository) Create(ctx context.Context, o Order) (Order, error) {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var couponCode *string
		if o.CouponCode != "" {
			couponCode = &o.CouponCode
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO orders (id, coupon_code, subtotal, discount, total) VALUES ($1, $2, $3, $4, $5)`,
			o.ID, couponCode, o.Subtotal, o.Discount, o.Total,
		); err != nil {
			return err
		}

		priceByProductID := make(map[string]float64, len(o.Products))
		for _, p := range o.Products {
			priceByProductID[p.ID] = p.Price
		}

		// All lines in one statement: unnest turns the three parallel arrays
		// into rows, so the cost doesn't grow with the number of items.
		ids := make([]string, len(o.Items))
		quantities := make([]int32, len(o.Items))
		prices := make([]float64, len(o.Items))
		for i, item := range o.Items {
			ids[i] = item.ProductID
			quantities[i] = int32(item.Quantity) //nolint:gosec // bounded by order.MaxItemQuantity
			prices[i] = priceByProductID[item.ProductID]
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, quantity, unit_price)
			 SELECT $1::uuid, line.product_id, line.quantity, line.unit_price
			 FROM unnest($2::text[], $3::int[], $4::numeric[]) AS line(product_id, quantity, unit_price)`,
			o.ID, ids, quantities, prices,
		)
		return err
	})
	if err != nil {
		return Order{}, err
	}
	return o, nil
}

var _ Repository = (*DBRepository)(nil)
