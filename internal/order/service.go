package order

import (
	"context"
	"time"

	"gofermart/internal/auth"
)

// Service проверяет и сохраняет заказы, получает результаты начисления бонусов.
// MakeOrder и GetOrders требуют контекст auth.Middleware.
type Service struct {
	Repository    Repository
	AccrualClient AccrualClient
	resumeOrderID string
}

// MakeOrder проверяет номер по Луну и сохраняет новый заказ со статусом NEW.
// Повторы возвращают ErrRepeatingOrder или ErrForeignOrder, в том числе при гонке вставок.
func (o *Service) MakeOrder(ctx context.Context, orderId string) error {
	if !PassesLuhn(orderId) {
		return ErrIncorrectLuhn
	}

	userId, ok := auth.UserID(ctx)
	if !ok {
		return auth.ErrInvalidCredentials
	}

	newOrder := Order{
		orderId:    orderId,
		userId:     userId,
		status:     StatusNew,
		uploadDate: time.Now(),
	}

	return o.Repository.SaveOrder(ctx, newOrder)
}

// GetOrders получает заказы текущего пользователя.
func (o *Service) GetOrders(ctx context.Context) ([]Order, error) {
	userId, _ := auth.UserID(ctx)
	return o.Repository.GetOrders(ctx, userId)
}
