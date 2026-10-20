package app

import (
	"net/http"

	"gofermart/internal/order"
	"gofermart/internal/user"
)

func newRouter(userHandler *user.Handler, orderHandler *order.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	userHandler.RegisterRoutes(mux)
	orderHandler.RegisterRoutes(mux)
	return mux
}
