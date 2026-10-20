package auth

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseToken(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)

	validClaims := jwt.RegisteredClaims{
		Subject: "42", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	valid := validTestToken(t, "42")
	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{name: "valid", token: valid},
		{name: "expired", token: signedTestToken(t, jwt.RegisteredClaims{
			Subject: "42", ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		}, jwt.SigningMethodHS256, []byte(testJWTSecret)), wantErr: jwt.ErrTokenExpired},
		{name: "missing expiration", token: signedTestToken(t, jwt.RegisteredClaims{
			Subject: "42",
		}, jwt.SigningMethodHS256, []byte(testJWTSecret)), wantErr: jwt.ErrTokenRequiredClaimMissing},
		{name: "wrong signature", token: signedTestToken(t, validClaims,
			jwt.SigningMethodHS256, []byte(strings.Repeat("x", 64))), wantErr: jwt.ErrTokenSignatureInvalid},
		{name: "wrong algorithm", token: signedTestToken(t, validClaims,
			jwt.SigningMethodHS512, []byte(testJWTSecret)), wantErr: jwt.ErrTokenSignatureInvalid},
		{name: "unsigned", token: signedTestToken(t, validClaims,
			jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType), wantErr: jwt.ErrTokenSignatureInvalid},
		{name: "not yet valid", token: signedTestToken(t, jwt.RegisteredClaims{
			Subject: "42", ExpiresAt: validClaims.ExpiresAt,
			NotBefore: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
		}, jwt.SigningMethodHS256, []byte(testJWTSecret)), wantErr: jwt.ErrTokenNotValidYet},
		{name: "malformed", token: "not-a-jwt", wantErr: jwt.ErrTokenMalformed},
		{name: "empty", token: "", wantErr: jwt.ErrTokenMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := ParseToken(tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if err != nil {
				if token != nil {
					t.Fatal("invalid token must not be returned to the caller")
				}
				return
			}
			if token == nil || !token.Valid {
				t.Fatal("expected a valid parsed token")
			}
			subject, err := token.Claims.GetSubject()
			if err != nil || subject != "42" {
				t.Fatalf("expected subject 42, got %q, error %v", subject, err)
			}
		})
	}
}

func TestTokenFunctionsRejectInvalidSecret(t *testing.T) {
	valid := validTestToken(t, "42")

	for _, secret := range []string{"", strings.Repeat("x", 31)} {
		t.Run(fmt.Sprintf("key length %d", len(secret)), func(t *testing.T) {
			t.Setenv("JWT_SECRET", secret)
			if token, err := IssueToken(42); err == nil || token != "" {
				t.Fatal("issuing a token must fail with an invalid key")
			}
			if token, err := ParseToken(valid); err == nil || token != nil {
				t.Fatal("verifying a token must fail with an invalid key")
			}
		})
	}
}
