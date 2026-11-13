package user

import (
	"context"

	"gofermart/internal/auth"
	"gofermart/internal/order"
)

// GetBalance возвращает баланс и сумму списаний текущего пользователя.
func (userService *Service) GetBalance(ctx context.Context) (float64, float64, error) {
	userID, _ := auth.UserID(ctx)
	return userService.Repository.GetBalance(ctx, userID)
}

// GetWithdrawals получает историю списаний текущего пользователя.
func (userService *Service) GetWithdrawals(ctx context.Context) ([]Withdrawal, error) {
	userID, _ := auth.UserID(ctx)
	return userService.Repository.GetWithdrawals(ctx, userID)
}

// Withdraw списывает баллы текущего пользователя.
// Возможны order.ErrIncorrectLuhn, ErrInvalidSum и ErrInsufficientFunds.
func (userService *Service) Withdraw(ctx context.Context, orderID string, sum float64) error {
	if len(orderID) > 255 || !order.PassesLuhn(orderID) {
		return order.ErrIncorrectLuhn
	}
	if sum <= 0 {
		return ErrInvalidSum
	}

	userID, _ := auth.UserID(ctx)
	return userService.Repository.Withdraw(ctx, Withdrawal{
		userID:  userID,
		orderID: orderID,
		sum:     sum,
	})
}
