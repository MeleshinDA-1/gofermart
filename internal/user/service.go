package user

import (
	"context"
	"errors"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"gofermart/internal/auth"
)

// Service выполняет регистрацию, вход, получение баланса и списания.
// Операции баланса и списаний требуют контекст auth.Middleware.
type Service struct {
	Repository Repository
}

// Authenticate проверяет логин и пароль и возвращает JWT.
// Неверные учётные данные приводят к auth.ErrInvalidCredentials.
func (userService *Service) Authenticate(ctx context.Context, login, password string) (string, error) {
	user, err := userService.Repository.GetUser(ctx, login)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", auth.ErrInvalidCredentials
	}

	if err != nil {
		return "", err
	}

	match, err := argon2id.ComparePasswordAndHash(password, user.PasswordHash)
	if err != nil {
		return "", err
	}

	if !match {
		return "", auth.ErrInvalidCredentials
	}

	return auth.IssueToken(user.ID)
}

// SaveUser хэширует пароль, регистрирует пользователя и возвращает JWT.
// При занятом логине возвращает ErrLoginTaken.
// Ошибка выпуска JWT не отменяет уже выполненное сохранение пользователя.
func (userService *Service) SaveUser(ctx context.Context, userApi Credentials) (string, error) {
	passwordHash, err := calculateHash(userApi.Password)
	if err != nil {
		return "", err
	}

	user := User{Login: userApi.Login, PasswordHash: passwordHash}

	userID, err := userService.Repository.SaveUser(ctx, user)
	if err != nil {
		return "", err
	}

	return auth.IssueToken(userID)
}

// NewService создаёт сервис пользователей с переданным репозиторием.
func NewService(userRepository Repository) *Service {
	return &Service{Repository: userRepository}
}
