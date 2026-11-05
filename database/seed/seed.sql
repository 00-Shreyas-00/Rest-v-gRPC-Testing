-- Deterministic product seed.
--
-- Every column is a pure function of the row number i: no random(), no now(),
-- no dependence on locale or server time zone. Running this script any number
-- of times produces a byte-identical table (verify with `scripts/db.sh verify`).
--
-- Usage (psql):  psql -v product_count=100000 -f seed.sql
-- product_count defaults to 100000.

\set ON_ERROR_STOP on
\if :{?product_count}
\else
    \set product_count 100000
\endif

BEGIN;

TRUNCATE products RESTART IDENTITY;

INSERT INTO products (id, name, description, category, price, stock, rating, created_at, updated_at)
SELECT
    i,
    -- e.g. "Nimbus Ergonomic Keyboard KE-000042" (unique per row)
    format('%s %s %s %s-%s',
           v.brands[1 + (i * 3) % 8],
           v.adjectives[1 + (i * 7) % 16],
           v.nouns[1 + i % 20],
           upper(left(v.nouns[1 + i % 20], 2)),
           lpad(i::text, 6, '0')),
    -- 1 to 4 sentences, so description length (and payload size) varies by row
    concat_ws(' ',
        format('The %s %s %s is designed for %s.',
               v.brands[1 + (i * 3) % 8],
               lower(v.adjectives[1 + (i * 7) % 16]),
               lower(v.nouns[1 + i % 20]),
               v.uses[1 + i % 6]),
        -- consecutive offsets from a per-row start, so a row never repeats a sentence
        CASE WHEN i % 4 >= 1 THEN v.features[1 + (i * 5)     % 8] END,
        CASE WHEN i % 4 >= 2 THEN v.features[1 + (i * 5 + 1) % 8] END,
        CASE WHEN i % 4 =  3 THEN v.features[1 + (i * 5 + 2) % 8] END),
    v.categories[1 + i % 20],
    round(((i * 7919) % 99900 + 99) / 100.0, 2),          -- 0.99 .. 999.98
    ((i * 37) % 1001)::int,                                -- 0 .. 1000
    ((i * 13) % 41 + 10) / 10.0,                           -- 1.0 .. 5.0
    timestamptz '2024-01-01 00:00:00+00' + make_interval(mins => (i * 5)::int),
    timestamptz '2024-01-01 00:00:00+00' + make_interval(mins => (i * 5)::int)
                                         + make_interval(hours => ((i * 7) % 720)::int)
FROM generate_series(1::bigint, :product_count::bigint) AS i
CROSS JOIN (
    SELECT
        ARRAY['Keyboard','Phone','Laptop','Monitor','Mouse',
              'Headphones','Speaker','Tablet','Camera','Charger',
              'Router','Webcam','Microphone','Smartwatch','Printer',
              'Drive','Cable','Dock','Controller','Projector'] AS nouns,
        ARRAY['Peripherals','Mobile','Computers','Displays','Peripherals',
              'Audio','Audio','Mobile','Cameras','Accessories',
              'Networking','Cameras','Audio','Wearables','Office',
              'Storage','Accessories','Accessories','Gaming','Displays'] AS categories,
        ARRAY['Wireless','Compact','Ergonomic','Portable','Professional','Ultra',
              'Classic','Smart','Rugged','Slim','Premium','Essential',
              'Advanced','Mini','Max','Eco'] AS adjectives,
        ARRAY['Acme','Nimbus','Vertex','Orion','Zenith','Helix','Quanta','Lumen'] AS brands,
        ARRAY['everyday use','home offices','travel','students',
              'professionals','small businesses'] AS uses,
        ARRAY['It ships with a two-year limited warranty.',
              'The housing is made from recycled aluminium.',
              'Setup takes less than five minutes.',
              'It is compatible with Windows, macOS and Linux.',
              'A USB-C connection provides power and data.',
              'Firmware updates are delivered automatically.',
              'The package includes a quick-start guide.',
              'It has been tested for over 10,000 hours of continuous operation.'] AS features
) AS v;

-- Keep the identity sequence ahead of the seeded ids so POST /products works.
SELECT setval(pg_get_serial_sequence('products', 'id'), (SELECT COALESCE(max(id), 1) FROM products)) \g /dev/null

COMMIT;

-- Fresh planner statistics and visibility map so every benchmark run starts from
-- the same physical state (must run outside a transaction block).
VACUUM (ANALYZE) products;
