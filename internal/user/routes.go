package user

import (
	"net/http"

	"gofermart/internal/auth"
)

// RegisterRoutes подключает регистрацию, вход и защищённые маршруты баланса и списаний.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/user/register", h.Register)
	mux.HandleFunc("POST /api/user/login", h.Login)
	mux.Handle("GET /api/user/balance", auth.Middleware(http.HandlerFunc(h.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", auth.Middleware(http.HandlerFunc(h.Withdraw)))
	mux.Handle("GET /api/user/withdrawals", auth.Middleware(http.HandlerFunc(h.GetWithdrawals)))
}
