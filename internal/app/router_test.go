package app

import (
	"net/http/httptest"
	"strings"
	"testing"

	"gofermart/internal/order"
	"gofermart/internal/user"
)

func TestRoutes(t *testing.T) {
	mux := newRouter(&user.Handler{}, &order.Handler{})
	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{"POST", "/api/user/register", 400},
		{"POST", "/api/user/login", 400},
		{"POST", "/api/user/orders", 401},
		{"GET", "/api/user/orders", 401},
		{"GET", "/api/user/balance", 401},
		{"POST", "/api/user/balance/withdraw", 401},
		{"GET", "/api/user/withdrawals", 401},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader("{"))
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
		})
	}
}
