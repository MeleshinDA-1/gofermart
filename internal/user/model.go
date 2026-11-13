package user

import (
	"time"
)

// User представляет пользователя, хранящегося в базе данных.
type User struct {
	ID    int64
	Login string
	// PasswordHash - хэш Argon2id вместе с солью и параметрами алгоритма.
	PasswordHash string
}

// Credentials содержит учётные данные из JSON-тела запроса регистрации или входа.
type Credentials struct {
	Login    string
	Password string
}

// Withdrawal хранит данные о списании баллов пользователем в счёт оплаты заказа.
type Withdrawal struct {
	id          int64
	userID      int64
	orderID     string
	sum         float64
	processedAt time.Time
}
