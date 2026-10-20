package auth

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// IssueToken выпускает JWT HS256 с ID пользователя в sub и сроком действия 24 часа.
func IssueToken(userId int64) (string, error) {
	secret, err := getJwtSecret()
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userId, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	})

	return token.SignedString([]byte(secret))
}

// ParseToken проверяет подпись HS256 и обязательный срок exp у JWT без префикса Bearer.
// Возвращает токен с *jwt.RegisteredClaims.
func ParseToken(tokenString string) (*jwt.Token, error) {
	secret, err := getJwtSecret()
	if err != nil {
		return nil, err
	}

	claims := &jwt.RegisteredClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return nil, err
	}

	return token, nil
}

func getJwtSecret() (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return "", errors.New("JWT_SECRET должен содержать минимум 32 байта")
	}

	return secret, nil
}
