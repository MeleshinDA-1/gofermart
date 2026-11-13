package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/alexedwards/argon2id"
	"github.com/jackc/pgx/v5"
	"gofermart/internal/auth"
)

func TestUserServiceAuthenticate(t *testing.T) {
	hash, err := calculateHash("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	dbErr := errors.New("database unavailable")
	tests := []struct {
		name      string
		password  string
		hash      string
		lookupErr error
		wantErr   error
		wantFail  bool
	}{
		{name: "success", password: "correct-password", hash: hash},
		{name: "wrong password", password: "wrong-password", hash: hash, wantErr: auth.ErrInvalidCredentials, wantFail: true},
		{name: "unknown login", lookupErr: fmt.Errorf("lookup: %w", pgx.ErrNoRows), wantErr: auth.ErrInvalidCredentials, wantFail: true},
		{name: "database failure", lookupErr: dbErr, wantErr: dbErr, wantFail: true},
		{name: "invalid stored hash", password: "correct-password", hash: "invalid-hash", wantFail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := User{ID: 9223372036854775807, Login: "alice", PasswordHash: tt.hash}
			service := testService(func(_ context.Context, _ string, args ...any) pgx.Row {
				if len(args) != 1 || args[0] != user.Login {
					t.Fatalf("unexpected login lookup arguments: %v", args)
				}
				return userRow(user, tt.lookupErr)
			})
			token, err := service.Authenticate(context.Background(), user.Login, tt.password)
			if tt.wantFail {
				if err == nil || token != "" {
					t.Fatalf("expected an error and no token, got token %q, error %v", token, err)
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			checkIssuedToken(t, token, user.ID)
		})
	}
}

func TestUserServiceSaveUserHashesPassword(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := Credentials{Login: "alice", Password: "registration-password"}
	var savedHash string
	service := testService(func(gotCtx context.Context, _ string, args ...any) pgx.Row {
		if gotCtx != ctx || len(args) != 2 || args[0] != input.Login {
			t.Fatal("registration context or login was not forwarded")
		}
		savedHash = args[1].(string)
		return idRow(42, nil)
	})

	token, err := service.SaveUser(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if savedHash == "" || savedHash == input.Password || !strings.HasPrefix(savedHash, "$argon2id$") {
		t.Fatal("repository must receive an Argon2id hash instead of the password")
	}
	match, err := argon2id.ComparePasswordAndHash(input.Password, savedHash)
	if err != nil || !match {
		t.Fatalf("saved hash does not match the password: %v", err)
	}
	checkIssuedToken(t, token, 42)
}
