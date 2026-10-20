package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func configureDb(databaseURI string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.TODO(), databaseURI)
	if err != nil {
		return nil, err
	}

	return pool, nil
}
