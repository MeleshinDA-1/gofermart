package auth

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// IssueToken выпускает JWT HS256 с ID пользователя в sub и сроком действия 24 часа.
func IssueToken(userID int64, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	})

	return token.SignedString([]byte(secret))
}

// ParseToken проверяет подпись HS256 и обязательный срок exp у JWT без префикса Bearer.
// Возвращает токен с *jwt.RegisteredClaims.
func ParseToken(tokenString, secret string) (*jwt.Token, error) {
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
