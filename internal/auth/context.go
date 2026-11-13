package auth

import (
	"context"
)

type userIDContextKey struct{}

// UserID возвращает ID из Middleware или (0, false), если ID в контексте нет.
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDContextKey{}).(int64)
	return id, ok
}
