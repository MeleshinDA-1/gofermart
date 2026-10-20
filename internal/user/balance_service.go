package user

import (
	"context"

	"gofermart/internal/auth"
	"gofermart/internal/order"
)

// GetBalance возвращает баланс и сумму списаний текущего пользователя.
func (userService *Service) GetBalance(ctx context.Context) (float64, float64, error) {
	userId, _ := auth.UserID(ctx)
	return userService.Repository.GetBalance(ctx, userId)
}

// GetWithdrawals получает историю списаний текущего пользователя.
func (userService *Service) GetWithdrawals(ctx context.Context) ([]Withdrawal, error) {
	userId, _ := auth.UserID(ctx)
	return userService.Repository.GetWithdrawals(ctx, userId)
}

// Withdraw списывает баллы текущего пользователя.
// Возможны order.ErrIncorrectLuhn, ErrInvalidSum и ErrInsufficientFunds.
func (userService *Service) Withdraw(ctx context.Context, orderId string, sum float64) error {
	if !order.PassesLuhn(orderId) {
		return order.ErrIncorrectLuhn
	}
	if sum <= 0 {
		return ErrInvalidSum
	}

	userId, _ := auth.UserID(ctx)
	return userService.Repository.Withdraw(ctx, Withdrawal{
		userId:  userId,
		orderId: orderId,
		sum:     sum,
	})
}
