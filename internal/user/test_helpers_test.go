package user

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type queryRowFunc func(context.Context, string, ...any) pgx.Row

func (query queryRowFunc) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return query(ctx, sql, args...)
}

func (query queryRowFunc) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("unexpected rows query")
}

func (query queryRowFunc) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return nil, errors.New("unexpected transaction")
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

func userRow(user User, err error) pgx.Row {
	return scanFunc(func(dest ...any) error {
		if err != nil {
			return err
		}
		*dest[0].(*int64) = user.ID
		*dest[1].(*string) = user.Login
		*dest[2].(*string) = user.PasswordHash
		return nil
	})
}

func testService(query queryRowFunc) *Service {
	return NewService(Repository{pool: query}, testJWTSecret)
}

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

func checkIssuedToken(t *testing.T, tokenString string, userID int64) {
	t.Helper()
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (any, error) {
		return []byte(testJWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(),
		jwt.WithSubject(strconv.FormatInt(userID, 10)))
	if err != nil {
		t.Fatalf("invalid issued token: %v", err)
	}
	if !token.Valid {
		t.Fatal("issued token is not valid")
	}
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining < 24*time.Hour-time.Minute || remaining > 24*time.Hour {
		t.Fatalf("expected a 24-hour token, remaining lifetime is %s", remaining)
	}
}

func testMux(handler *Handler) *http.ServeMux {
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, testJWTSecret)
	return mux
}
