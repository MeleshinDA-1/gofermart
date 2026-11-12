package order

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"gofermart/internal/auth"
)

type orderInsertDB struct {
	ordersDB
	exec     func(context.Context, string, ...any) (pgconn.CommandTag, error)
	queryRow func(context.Context, string, ...any) pgx.Row
}

func (db orderInsertDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return db.exec(ctx, sql, args...)
}

func (db orderInsertDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.queryRow(ctx, sql, args...)
}

func TestOrderInsertConflictHTTP(t *testing.T) {
	token := validTestToken(t, "7")
	for _, test := range []struct {
		name       string
		inserted   bool
		ownerID    int64
		insertErr  error
		readErr    error
		wantStatus int
	}{
		{name: "new order", inserted: true, wantStatus: 202},
		{name: "same owner won insert", ownerID: 7, wantStatus: 200},
		{name: "other owner won insert", ownerID: 8, wantStatus: 409},
		{name: "database failure", insertErr: errors.New("database unavailable"), wantStatus: 500},
		{name: "unrelated constraint", insertErr: &pgconn.PgError{Code: "23505", ConstraintName: "other_key"}, wantStatus: 500},
		{name: "owner lookup failure", readErr: errors.New("read failed"), wantStatus: 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			readCalls, insertCalls := 0, 0
			started := time.Now()
			db := orderInsertDB{
				exec: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
					insertCalls++
					if args[0] != "12345678903" || args[1] != int64(7) {
						t.Fatalf("unexpected insert arguments: %v", args)
					}
					if args[2] != "NEW" || args[3] != float64(0) {
						t.Fatalf("unexpected initial status or accrual: %v", args)
					}
					uploadedAt := args[4].(time.Time)
					if uploadedAt.Before(started) || uploadedAt.After(time.Now()) {
						t.Fatalf("unexpected upload time: %v", uploadedAt)
					}
					if test.inserted {
						return pgconn.NewCommandTag("INSERT 0 1"), nil
					}
					return pgconn.NewCommandTag("INSERT 0 0"), test.insertErr
				},
				queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
					readCalls++
					if len(args) != 1 || args[0] != "12345678903" {
						t.Fatalf("unexpected lookup arguments: %v", args)
					}
					return idRow(test.ownerID, test.readErr)
				},
			}
			api := Handler{Service: Service{Repository: Repository{pool: db}}}

			handler := auth.Middleware(http.HandlerFunc(api.MakeOrder), testJWTSecret)
			request := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("12345678903"))
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Body.Len() != 0 {
				t.Fatalf("unexpected response body: %s", response.Body.String())
			}
			wantReads := 1
			if test.inserted || test.insertErr != nil {
				wantReads = 0
			}
			if insertCalls != 1 || readCalls != wantReads {
				t.Fatalf("insert calls = %d, read calls = %d", insertCalls, readCalls)
			}
		})
	}
}
