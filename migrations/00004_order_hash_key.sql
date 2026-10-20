-- +goose Up
ALTER TABLE orders
    ADD COLUMN orderHash BYTEA GENERATED ALWAYS AS (sha256(orderId::bytea)) STORED;
ALTER TABLE orders DROP CONSTRAINT orders_pkey;
ALTER TABLE orders ADD PRIMARY KEY (orderHash);

-- +goose Down
ALTER TABLE orders DROP CONSTRAINT orders_pkey;
ALTER TABLE orders ADD PRIMARY KEY (orderId);
ALTER TABLE orders DROP COLUMN orderHash;
