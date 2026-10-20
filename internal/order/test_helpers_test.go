package order

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func signedTestToken(t *testing.T, claims jwt.Claims, method jwt.SigningMethod, key any) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func validTestToken(t *testing.T, subject string) string {
	t.Helper()
	return signedTestToken(t, jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}, jwt.SigningMethodHS256, []byte(testJWTSecret))
}

type scanFunc func(...any) error

func (scan scanFunc) Scan(dest ...any) error {
	return scan(dest...)
}

func idRow(id int64, err error) pgx.Row {
	return scanFunc(func(dest ...any) error {
		if err != nil {
			return err
		}
		*dest[0].(*int64) = id
		return nil
	})
}
