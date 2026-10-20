package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCheckingToken(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	valid := validTestToken(t, "42")
	claims := jwt.RegisteredClaims{Subject: "42", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}
	expired := signedTestToken(t, jwt.RegisteredClaims{
		Subject: "42", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}, jwt.SigningMethodHS256, []byte(testJWTSecret))
	forged := signedTestToken(t, claims, jwt.SigningMethodHS256, []byte(strings.Repeat("x", 64)))
	noExpiration := signedTestToken(t, jwt.RegisteredClaims{Subject: "42"}, jwt.SigningMethodHS256, []byte(testJWTSecret))
	wrongAlgorithm := signedTestToken(t, claims, jwt.SigningMethodHS512, []byte(testJWTSecret))
	unsigned := signedTestToken(t, claims, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType)
	tests := []struct {
		name          string
		authorization string
		wantID        int64
	}{
		{name: "valid", authorization: "Bearer " + valid, wantID: 42},
		{name: "case insensitive scheme", authorization: "bEaReR " + valid, wantID: 42},
		{name: "another user", authorization: "Bearer " + validTestToken(t, "73"), wantID: 73},
		{name: "missing header"},
		{name: "missing separator", authorization: "Bearer"},
		{name: "empty token", authorization: "Bearer "},
		{name: "wrong scheme", authorization: "Basic " + valid},
		{name: "bare token", authorization: valid},
		{name: "malformed token", authorization: "Bearer not-a-jwt"},
		{name: "expired token", authorization: "Bearer " + expired},
		{name: "wrong signature", authorization: "Bearer " + forged},
		{name: "missing expiration", authorization: "Bearer " + noExpiration},
		{name: "wrong algorithm", authorization: "Bearer " + wrongAlgorithm},
		{name: "unsigned", authorization: "Bearer " + unsigned},
		{name: "missing subject", authorization: "Bearer " + validTestToken(t, "")},
		{name: "non numeric subject", authorization: "Bearer " + validTestToken(t, "alice")},
		{name: "zero subject", authorization: "Bearer " + validTestToken(t, "0")},
		{name: "negative subject", authorization: "Bearer " + validTestToken(t, "-1")},
		{name: "overflowing subject", authorization: "Bearer " + validTestToken(t, "9223372036854775808")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			request := httptest.NewRequest(http.MethodGet, "/protected", nil).WithContext(ctx)
			request.Header.Set("Authorization", tt.authorization)
			response := httptest.NewRecorder()
			calls := 0

			Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				userID, ok := r.Context().Value(userIDContextKey{}).(int64)
				if !ok || userID != tt.wantID {
					t.Fatalf("expected user ID %d in context, got %v", tt.wantID, r.Context().Value(userIDContextKey{}))
				}
				cancel()
				if !errors.Is(r.Context().Err(), context.Canceled) {
					t.Fatal("middleware lost the original request cancellation")
				}
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(response, request)

			if tt.wantID == 0 {
				if response.Code != http.StatusUnauthorized || calls != 0 {
					t.Fatalf("invalid authorization: status %d, next called %d times", response.Code, calls)
				}
			} else if response.Code != http.StatusNoContent || calls != 1 {
				t.Fatalf("valid authorization: status %d, next called %d times", response.Code, calls)
			}
			if request.Context().Value(userIDContextKey{}) != nil {
				t.Fatal("middleware modified the original request context")
			}
		})
	}
}
