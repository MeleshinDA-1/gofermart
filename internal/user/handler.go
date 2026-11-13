package user

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"unicode/utf8"

	"gofermart/internal/auth"
)

// Handler обрабатывает HTTP-запросы пользователей. Неверный JSON даёт 400, внутренние ошибки - 500.
// Register и Login принимают JSON Credentials и при успехе возвращают 200 и Authorization: Bearer <JWT>.
// Баланс и списания требуют auth.Middleware.
type Handler struct {
	Service Service
}

// Register регистрирует пользователя; занятый логин даёт 409.
func (h Handler) Register(responseWriter http.ResponseWriter, request *http.Request) {
	var requestUser Credentials

	if err := json.NewDecoder(request.Body).Decode(&requestUser); err != nil {
		http.Error(
			responseWriter,
			"Некорректный JSON",
			http.StatusBadRequest,
		)
		return
	}

	if utf8.RuneCountInString(requestUser.Login) > 255 {
		http.Error(responseWriter, "Логин длиннее 255 символов", http.StatusBadRequest)
		return
	}

	token, err := h.Service.SaveUser(
		request.Context(),
		requestUser,
	)

	if errors.Is(err, ErrLoginTaken) {
		http.Error(responseWriter, "Логин уже занят", http.StatusConflict)
		return
	}

	if err != nil {
		log.Printf("Регистрация пользователя: %v", err)
		http.Error(responseWriter, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	responseWriter.Header().Set("Authorization", "Bearer "+token)
	responseWriter.WriteHeader(http.StatusOK)
}

// Login аутентифицирует пользователя; неверные учётные данные дают 401.
func (h Handler) Login(responseWriter http.ResponseWriter, request *http.Request) {
	var requestUser Credentials
	if err := json.NewDecoder(request.Body).Decode(&requestUser); err != nil {
		http.Error(
			responseWriter,
			"Некорректный JSON",
			http.StatusBadRequest,
		)
		return
	}

	if utf8.RuneCountInString(requestUser.Login) > 255 {
		http.Error(responseWriter, "Логин длиннее 255 символов", http.StatusBadRequest)
		return
	}

	token, err := h.Service.Authenticate(request.Context(), requestUser.Login, requestUser.Password)

	if errors.Is(err, auth.ErrInvalidCredentials) {
		http.Error(responseWriter, "Неправильные логин или пароль", http.StatusUnauthorized)
		return
	}

	if err != nil {
		log.Printf("Аутентификация пользователя: %v", err)
		http.Error(responseWriter, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	responseWriter.Header().Set("Authorization", "Bearer "+token)
	responseWriter.WriteHeader(http.StatusOK)
}

// NewHandler создаёт HTTP-обработчики, использующие переданный сервис пользователей.
func NewHandler(userService Service) *Handler {
	return &Handler{Service: userService}
}
