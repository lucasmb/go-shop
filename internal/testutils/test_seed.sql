-- This seed data is ONLY for running automated tests.
-- It provides a small, stable, and predictable dataset matching the main schema.

-- Clear existing data to ensure a clean slate for each test run if needed,
-- though the in-memory DB is recreated each time.
DELETE FROM categories;
DELETE FROM products;

-- Insert categories first due to foreign key constraints.
INSERT INTO categories (id, name) VALUES (1, 'Apparel'), (2, 'Books'), (3, 'Accessories');

-- Insert products with all required NOT NULL fields:
-- id, name, description, price, stock, created_at, image_url
INSERT INTO products (id, name, description, price, category_id, stock, created_at, image_url, variants_json) VALUES
(1, 'Go T-Shirt', 'A blue t-shirt for Go developers.', 2999, 1, 10, '2023-01-01 10:00:00', 'https://placehold.co/600x400/00ADD8/FFFFFF/png?text=Go+T-Shirt',
'{
  "variant_groups": [{"name": "Size", "options": ["Small", "Medium"]}],
  "skus": [
    {"attributes": {"Size": "Small"}, "priceModifier": 0, "stock": 5},
    {"attributes": {"Size": "Medium"}, "priceModifier": 200, "stock": 5}
  ]
}'),
(2, 'Go Book', 'The official Go programming language book.', 3999, 2, 15, '2023-02-01 10:00:00', 'https://placehold.co/600x400/7C3AED/FFFFFF/png?text=Go+Book', NULL),
(3, 'HTMX T-Shirt', 'A black t-shirt for HTMX fans.', 2499, 1, 20, '2023-03-01 10:00:00', 'https://placehold.co/600x400/3366CC/FFFFFF/png?text=HTMX+T-Shirt',
'{
  "variant_groups": [{"name": "Size", "options": ["Medium", "Large"]}],
  "skus": [
    {"attributes": {"Size": "Medium"}, "priceModifier": 0, "stock": 10},
    {"attributes": {"Size": "Large"}, "priceModifier": 300, "stock": 10}
  ]
}'),
(4, 'Performance Book', 'A book about high-performance Go.', 4500, 2, 5, '2022-12-01 10:00:00', 'https://placehold.co/600x400/10B981/FFFFFF/png?text=Perf+Book', NULL),
(5, 'Go Gopher Pin', 'A stylish enamel pin.', 799, 3, 100, '2023-04-01 10:00:00', 'https://placehold.co/600x400/DB2777/FFFFFF/png?text=Pin', NULL);
