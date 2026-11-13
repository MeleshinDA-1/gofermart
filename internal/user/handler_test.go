package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"gofermart/internal/auth"
)

func TestUserEndpoints(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	hash, err := calculateHash("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	const credentials = `{"login":"alice","password":"correct-password"}`
	dbErr := errors.New("private database error")
	tests := []struct {
		name       string
		login      string
		path       string
		body       string
		method     string
		rowErr     error
		badHash    bool
		wantStatus int
		wantCalls  int
	}{
		{name: "register success", path: "/api/user/register", body: credentials, wantStatus: 200, wantCalls: 1},
		{name: "register maximum login", login: strings.Repeat("я", 255), path: "/api/user/register", body: `{"login":"` + strings.Repeat("я", 255) + `","password":"correct-password"}`, wantStatus: 200, wantCalls: 1},
		{name: "register malformed JSON", path: "/api/user/register", body: "{", wantStatus: 400},
		{name: "register empty body", path: "/api/user/register", wantStatus: 400},
		{name: "register invalid field type", path: "/api/user/register", body: `{"login":123}`, wantStatus: 400},
		{name: "register login too long", path: "/api/user/register", body: `{"login":"` + strings.Repeat("я", 256) + `","password":"password"}`, wantStatus: 400},
		{name: "register duplicate login", path: "/api/user/register", body: credentials,
			rowErr: &pgconn.PgError{Code: "23505", ConstraintName: "users_login_key"}, wantStatus: 409, wantCalls: 1},
		{name: "register database failure", path: "/api/user/register", body: credentials, rowErr: dbErr, wantStatus: 500, wantCalls: 1},
		{name: "login success", path: "/api/user/login", body: credentials, wantStatus: 200, wantCalls: 1},
		{name: "login maximum length", login: strings.Repeat("я", 255), path: "/api/user/login", body: `{"login":"` + strings.Repeat("я", 255) + `","password":"correct-password"}`, wantStatus: 200, wantCalls: 1},
		{name: "login malformed JSON", path: "/api/user/login", body: "{", wantStatus: 400},
		{name: "login empty body", path: "/api/user/login", wantStatus: 400},
		{name: "login invalid field type", path: "/api/user/login", body: `{"password":123}`, wantStatus: 400},
		{name: "login too long", path: "/api/user/login", body: `{"login":"` + strings.Repeat("я", 256) + `","password":"password"}`, wantStatus: 400},
		{name: "login unknown user", path: "/api/user/login", body: credentials, rowErr: pgx.ErrNoRows, wantStatus: 401, wantCalls: 1},
		{name: "login wrong password", path: "/api/user/login", body: `{"login":"alice","password":"wrong-password"}`, wantStatus: 401, wantCalls: 1},
		{name: "login database failure", path: "/api/user/login", body: credentials, rowErr: dbErr, wantStatus: 500, wantCalls: 1},
		{name: "login corrupt hash", path: "/api/user/login", body: credentials, badHash: true, wantStatus: 500, wantCalls: 1},
		{name: "register wrong method", path: "/api/user/register", method: http.MethodGet, wantStatus: 405},
		{name: "login wrong method", path: "/api/user/login", method: http.MethodGet, wantStatus: 405},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			login := tt.login
			if login == "" {
				login = "alice"
			}
			request := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			if tt.method != "" {
				request.Method = tt.method
			}
			service := testService(func(ctx context.Context, _ string, args ...any) pgx.Row {
				calls++
				if ctx != request.Context() {
					t.Fatal("HTTP request context was not forwarded to the database")
				}
				if len(args) == 0 || args[0] != login {
					t.Fatalf("unexpected login argument: %v", args)
				}
				if tt.path == "/api/user/register" {
					return idRow(42, tt.rowErr)
				}
				storedHash := hash
				if tt.badHash {
					storedHash = "invalid-hash"
				}
				return userRow(User{ID: 42, Login: login, PasswordHash: storedHash}, tt.rowErr)
			})
			api := NewHandler(*service)
			response := httptest.NewRecorder()
			testMux(api).ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, response.Code, response.Body.String())
			}
			if calls != tt.wantCalls {
				t.Fatalf("expected %d database calls, got %d", tt.wantCalls, calls)
			}
			for _, privateValue := range []string{"correct-password", "wrong-password", hash, dbErr.Error()} {
				if strings.Contains(response.Body.String(), privateValue) {
					t.Fatalf("response exposes private value %q", privateValue)
				}
			}
			authorization := response.Header().Get("Authorization")
			if tt.wantStatus != http.StatusOK {
				if authorization != "" {
					t.Fatal("failed request must not receive an authorization token")
				}
				return
			}

			token, ok := strings.CutPrefix(authorization, "Bearer ")
			if !ok {
				t.Fatalf("missing Bearer token: %q", authorization)
			}
			checkIssuedToken(t, token, 42)
			protectedRequest := httptest.NewRequest(http.MethodGet, "/protected", nil)
			protectedRequest.Header.Set("Authorization", authorization)
			protectedResponse := httptest.NewRecorder()
			called := false
			auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if id, ok := auth.UserID(r.Context()); !ok || id != 42 {
					t.Fatalf("issued token authenticated unexpected user: %v", id)
				}
				w.WriteHeader(http.StatusNoContent)
			}), testJWTSecret).ServeHTTP(protectedResponse, protectedRequest)
			if !called || protectedResponse.Code != http.StatusNoContent {
				t.Fatalf("issued token did not authorize the next request: %d", protectedResponse.Code)
			}
		})
	}
}
