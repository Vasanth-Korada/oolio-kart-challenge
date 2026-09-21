CREATE TABLE IF NOT EXISTS orders (
    id          UUID PRIMARY KEY,
    coupon_code TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_items (
    order_id   UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id TEXT NOT NULL REFERENCES products (id),
    quantity   INTEGER NOT NULL CHECK (quantity > 0),
    -- Price snapshot at order time — never re-derived from the current
    -- product price, so a later price change can't rewrite history.
    unit_price NUMERIC(10, 2) NOT NULL CHECK (unit_price >= 0),
    PRIMARY KEY (order_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items (order_id);
