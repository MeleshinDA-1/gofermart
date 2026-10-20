-- +goose Up
CREATE TABLE withdrawals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    userId BIGINT NOT NULL REFERENCES users(id),
    orderId TEXT NOT NULL,
    sum NUMERIC NOT NULL,
    processedAt TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE withdrawals;
