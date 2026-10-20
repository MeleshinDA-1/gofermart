package order

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/jackc/pgx/v5/pgxpool"
	"gofermart/internal/auth"
)

func TestOrderUploadErrors(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	pool, err := pgxpool.New(context.Background(), "postgres://localhost/gofermart")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	api := Handler{Service: Service{Repository: Repository{pool: pool}}}

	handler := auth.Middleware(http.HandlerFunc(api.MakeOrder))
	token := validTestToken(t, "1")
	tests := []struct {
		name       string
		body       io.Reader
		authorized bool
		wantStatus int
	}{
		{name: "unauthorized", body: strings.NewReader("12345678903"), wantStatus: 401},
		{name: "unreadable body", body: iotest.ErrReader(io.ErrUnexpectedEOF), authorized: true, wantStatus: 400},
		{name: "empty number", body: strings.NewReader(""), authorized: true, wantStatus: 422},
		{name: "bad checksum", body: strings.NewReader("12345678904"), authorized: true, wantStatus: 422},
		{name: "database unavailable", body: strings.NewReader("12345678903"), authorized: true, wantStatus: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/user/orders", test.body)
			if test.authorized {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
	if _, err := api.Service.pollBonuses(context.Background()); err == nil {
		t.Fatal("poller ignored database failure")
	}
}
