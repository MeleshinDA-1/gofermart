package user

import (
	"errors"
)

// ErrLoginTaken означает, что пользователь с таким логином уже существует.
var ErrLoginTaken = errors.New("login already taken")

// ErrInsufficientFunds означает, что доступных баллов недостаточно для списания.
var ErrInsufficientFunds = errors.New("insufficient funds")

// ErrInvalidSum означает, что запрошенная сумма списания не положительна.
var ErrInvalidSum = errors.New("withdrawal sum must be positive")
