package order

import (
	"net/http"

	"gofermart/internal/auth"
)

// RegisterRoutes подключает загрузку и чтение заказов через проверку JWT.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, jwtSecret string) {
	mux.Handle("POST /api/user/orders", auth.Middleware(http.HandlerFunc(h.MakeOrder), jwtSecret))
	mux.Handle("GET /api/user/orders", auth.Middleware(http.HandlerFunc(h.GetOrders), jwtSecret))
}
