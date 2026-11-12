package app

import (
	"net/http"

	"gofermart/internal/order"
	"gofermart/internal/user"
)

func newRouter(userHandler *user.Handler, orderHandler *order.Handler, jwtSecret string) *http.ServeMux {
	mux := http.NewServeMux()
	userHandler.RegisterRoutes(mux, jwtSecret)
	orderHandler.RegisterRoutes(mux, jwtSecret)
	return mux
}
