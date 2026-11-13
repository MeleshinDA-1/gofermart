package app

import (
	"context"
	"fmt"

	"gofermart/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func configureDB(databaseURI string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.TODO(), databaseURI)
	if err != nil {
		return nil, err
	}
	if err := migrations.Up(context.Background(), pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return pool, nil
}
