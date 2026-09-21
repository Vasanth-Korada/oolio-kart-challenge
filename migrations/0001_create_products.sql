CREATE TABLE IF NOT EXISTS products (
    id       TEXT PRIMARY KEY,
    name     TEXT NOT NULL,
    price    NUMERIC(10, 2) NOT NULL CHECK (price >= 0),
    category TEXT NOT NULL
);

-- The spec has no product-creation endpoint, so the catalog is seeded here.
INSERT INTO products (id, name, price, category) VALUES
    ('1',  'Waffle with Berries',            6.50, 'Waffle'),
    ('2',  'Vanilla Bean Crème Brûlée',      7.00, 'Crème Brûlée'),
    ('3',  'Macaron Mix of Five',            8.00, 'Macaron'),
    ('4',  'Classic Tiramisu',               5.50, 'Tiramisu'),
    ('5',  'Berry Basque Burnt Cheesecake',  6.50, 'Cheesecake'),
    ('6',  'Salted Caramel Macaron',         8.00, 'Macaron'),
    ('7',  'Chocolate Souffle',              6.00, 'Souffle'),
    ('8',  'Vanilla Panna Cotta',            6.00, 'Panna Cotta'),
    ('9',  'Oat & Raisin Cookie',            3.50, 'Cookie'),
    ('10', 'Chicken Waffle',                 9.00, 'Waffle')
ON CONFLICT (id) DO NOTHING;
