-- +goose Up
CREATE TABLE orders (
    orderId TEXT PRIMARY KEY,
    userId BIGINT NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'NEW',
    bonusesCredited NUMERIC,
    uploadDate TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE orders;