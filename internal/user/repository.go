package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository работает с пользователями и списаниями в PostgreSQL.
type Repository struct {
	pool userQuerier
}

type userQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error)
}

// SaveUser вставляет пользователя с готовым PasswordHash и возвращает назначенный БД ID.
// Занятый логин приводит к ErrLoginTaken.
func (userRepository Repository) SaveUser(ctx context.Context, user User) (int64, error) {
	var id int64

	err := userRepository.pool.QueryRow(
		ctx,
		`INSERT INTO users (login, passwordHash) VALUES ($1, $2) RETURNING id`, user.Login, user.PasswordHash,
	).Scan(&id)

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "users_login_key" {
		return 0, ErrLoginTaken
	}

	return id, err
}

// GetUser находит пользователя с хэшем пароля по логину; при отсутствии возвращает pgx.ErrNoRows.
func (userRepository Repository) GetUser(ctx context.Context, login string) (User, error) {
	var user User

	err := userRepository.pool.QueryRow(
		ctx,
		`SELECT id, login, passwordHash FROM users WHERE login = $1`, login,
	).Scan(&user.ID, &user.Login, &user.PasswordHash)

	return user, err
}

// NewRepository создаёт репозиторий с переданным инициализированным пулом.
// За закрытие пула после завершения работы отвечает вызывающий код.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}
