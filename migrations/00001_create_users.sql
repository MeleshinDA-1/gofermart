-- +goose Up
CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login VARCHAR(255) NOT NULL,
    passwordHash VARCHAR(255) NOT NULL,
    CONSTRAINT users_login_key UNIQUE (login)
);

-- +goose Down
DROP TABLE users;
