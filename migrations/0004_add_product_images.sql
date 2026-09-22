ALTER TABLE products
    ADD COLUMN IF NOT EXISTS image_thumbnail TEXT,
    ADD COLUMN IF NOT EXISTS image_mobile    TEXT,
    ADD COLUMN IF NOT EXISTS image_tablet    TEXT,
    ADD COLUMN IF NOT EXISTS image_desktop   TEXT;

-- Deterministic placeholder images (picsum.photos), keyed by product id.
-- The assignment never supplied real product photography.
UPDATE products SET
    image_thumbnail = 'https://picsum.photos/seed/product-' || id || '/150/150',
    image_mobile    = 'https://picsum.photos/seed/product-' || id || '/375/250',
    image_tablet    = 'https://picsum.photos/seed/product-' || id || '/600/400',
    image_desktop   = 'https://picsum.photos/seed/product-' || id || '/900/600'
WHERE image_thumbnail IS NULL;
