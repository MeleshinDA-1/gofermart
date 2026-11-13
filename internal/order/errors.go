package order

import (
	"errors"
)

// ErrIncorrectLuhn означает, что номер пуст, содержит не цифры или не прошёл проверку Луна.
var ErrIncorrectLuhn = errors.New("the order didn't pass the Luhn validation")

// ErrForeignOrder означает, что заказ уже загружен другим пользователем.
var ErrForeignOrder = errors.New("the order is foreign")

// ErrRepeatingOrder означает, что пользователь уже загрузил этот заказ.
var ErrRepeatingOrder = errors.New("you have already created the order")
