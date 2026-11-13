package auth

import (
	"errors"
)

// ErrInvalidCredentials означает, что логин не найден или пароль неверен.
var ErrInvalidCredentials = errors.New("invalid credentials")
