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
func (r *DBRepository) Create(ctx context.Context, order Order) (Order, error) {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var couponCode *string
		if order.CouponCode != "" {
			couponCode = &order.CouponCode
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO orders (id, coupon_code, subtotal, discount, total) VALUES ($1, $2, $3, $4, $5)`,
			order.ID, couponCode, order.Subtotal, order.Discount, order.Total,
		); err != nil {
			return err
		}

		priceByProductID := make(map[string]float64, len(order.Products))
		for _, product := range order.Products {
			priceByProductID[product.ID] = product.Price
		}

		// All lines in one statement: unnest turns the three parallel arrays
		// into rows, so the cost doesn't grow with the number of items.
		ids := make([]string, len(order.Items))
		quantities := make([]int32, len(order.Items))
		prices := make([]float64, len(order.Items))
		for lineIndex, item := range order.Items {
			ids[lineIndex] = item.ProductID
			quantities[lineIndex] = int32(item.Quantity) //nolint:gosec // bounded by order.MaxItemQuantity
			prices[lineIndex] = priceByProductID[item.ProductID]
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO order_items (order_id, product_id, quantity, unit_price)
			 SELECT $1::uuid, line.product_id, line.quantity, line.unit_price
			 FROM unnest($2::text[], $3::int[], $4::numeric[]) AS line(product_id, quantity, unit_price)`,
			order.ID, ids, quantities, prices,
		)
		return err
	})
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

var _ Repository = (*DBRepository)(nil)
