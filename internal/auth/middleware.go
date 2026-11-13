package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

// Middleware проверяет Authorization: Bearer <JWT> и положительный числовой sub.
// При ошибке отвечает 401, при успехе передаёт next контекст с ID, доступным через UserID.
func Middleware(next http.Handler, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		parsedToken, err := ParseToken(token, secret)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		subject, err := parsedToken.Claims.GetSubject()
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		userID, err := strconv.ParseInt(subject, 10, 64)
		if err != nil || userID <= 0 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
