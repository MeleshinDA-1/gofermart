// Package app собирает зависимости приложения, HTTP-маршруты и фоновый опрос начислений.
package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gofermart/internal/config"
	"gofermart/internal/order"
	"gofermart/internal/user"

	"golang.org/x/sync/errgroup"
)

// Run открывает пул PostgreSQL, подключает маршруты и запускает HTTP-сервер.
// Требует cfg.AccrualAddress; при завершении отменяет опрос и закрывает пул.
func Run(cfg config.Config) error {
	if cfg.AccrualAddress == "" {
		return errors.New("задайте ACCRUAL_SYSTEM_ADDRESS или флаг -r")
	}

	pool, err := configureDB(cfg.DatabaseURI)
	if err != nil {
		return err
	}
	defer pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userRepository := user.NewRepository(pool)
	userService := user.NewService(*userRepository, cfg.JWTSecret)
	userHandler := user.NewHandler(*userService)

	orderRepository := order.NewRepository(pool)
	accrualClient := order.NewAccrualClient(cfg.AccrualAddress, &http.Client{Timeout: 10 * time.Second})
	orderHandler := &order.Handler{Service: order.Service{
		Repository:    *orderRepository,
		AccrualClient: *accrualClient,
	}}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		orderHandler.Service.RunPoller(ctx)
		return nil
	})

	group.Go(func() error {
		return http.ListenAndServe(cfg.RunAddress, newRouter(userHandler, orderHandler, cfg.JWTSecret))
	})

	return group.Wait()
}
