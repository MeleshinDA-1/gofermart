-- +goose Up
CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'INVALID', 'PROCESSED');

CREATE TABLE orders (
    orderId VARCHAR(255) NOT NULL,
    userId BIGINT NOT NULL REFERENCES users(id),
    status order_status NOT NULL DEFAULT 'NEW',
    bonusesCredited NUMERIC,
    uploadDate TIMESTAMPTZ NOT NULL DEFAULT now(),
    orderHash BYTEA GENERATED ALWAYS AS (sha256(orderId::bytea)) STORED PRIMARY KEY
);

-- +goose Down
DROP TABLE orders;
DROP TYPE order_status;
