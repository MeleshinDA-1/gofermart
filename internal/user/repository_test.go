package user

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUserRepositorySaveUser(t *testing.T) {
	dbErr := errors.New("database unavailable")
	otherConstraint := &pgconn.PgError{Code: "23505", ConstraintName: "users_pkey"}
	otherCode := &pgconn.PgError{Code: "23502", ConstraintName: "users_login_key"}
	tests := []struct {
		name    string
		dbErr   error
		wantErr error
	}{
		{name: "success"},
		{name: "login conflict", dbErr: &pgconn.PgError{Code: "23505", ConstraintName: "users_login_key"}, wantErr: ErrLoginTaken},
		{name: "another unique constraint", dbErr: otherConstraint, wantErr: otherConstraint},
		{name: "another postgres error", dbErr: otherCode, wantErr: otherCode},
		{name: "database failure", dbErr: dbErr, wantErr: dbErr},
		{name: "canceled request", dbErr: context.Canceled, wantErr: context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if errors.Is(tt.dbErr, context.Canceled) {
				cancel()
			}
			calls := 0
			repository := Repository{pool: queryRowFunc(func(gotCtx context.Context, _ string, args ...any) pgx.Row {
				calls++
				if gotCtx != ctx {
					t.Fatal("request context was not forwarded")
				}
				if len(args) != 2 || args[0] != "alice" || args[1] != "stored-hash" {
					t.Fatalf("unexpected insert arguments: %v", args)
				}
				return idRow(42, tt.dbErr)
			})}

			id, err := repository.SaveUser(ctx, User{ID: 999, Login: "alice", PasswordHash: "stored-hash"})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if err == nil && id != 42 {
				t.Fatalf("expected database ID 42, got %d", id)
			}
			if calls != 1 {
				t.Fatalf("expected one insert, got %d", calls)
			}
		})
	}
}

func TestUserRepositoryGetUser(t *testing.T) {
	dbErr := errors.New("database unavailable")
	for _, wantErr := range []error{nil, pgx.ErrNoRows, dbErr, context.DeadlineExceeded} {
		name := "success"
		if wantErr != nil {
			name = wantErr.Error()
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := User{ID: 42, Login: "alice", PasswordHash: "stored-hash"}
			repository := Repository{pool: queryRowFunc(func(gotCtx context.Context, _ string, args ...any) pgx.Row {
				if gotCtx != ctx || len(args) != 1 || args[0] != want.Login {
					t.Fatal("lookup context or login was not forwarded")
				}
				return userRow(want, wantErr)
			})}

			got, err := repository.GetUser(ctx, want.Login)
			if !errors.Is(err, wantErr) {
				t.Fatalf("expected error %v, got %v", wantErr, err)
			}
			if err == nil && got != want {
				t.Fatalf("expected user %+v, got %+v", want, got)
			}
		})
	}
}
