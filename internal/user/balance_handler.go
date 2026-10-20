package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"gofermart/internal/order"
)

// GetBalance отвечает 200 с JSON: current - баланс, withdrawn - сумма списаний.
func (userApi Handler) GetBalance(responseWriter http.ResponseWriter, request *http.Request) {
	current, withdrawn, err := userApi.Service.GetBalance(request.Context())
	if err != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	balance := struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}{
		Current:   current,
		Withdrawn: withdrawn,
	}
	body, err := json.Marshal(balance)
	if err != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Write(body)
}

// GetWithdrawals отвечает 200 со своими списаниями от новых к старым: order, sum, processed_at (RFC3339).
// При пустом списке возвращает 204.
func (userApi Handler) GetWithdrawals(responseWriter http.ResponseWriter, request *http.Request) {
	withdrawals, err := userApi.Service.GetWithdrawals(request.Context())
	if err != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(withdrawals) == 0 {
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}

	type withdrawalResponse struct {
		Order       string  `json:"order"`
		Sum         float64 `json:"sum"`
		ProcessedAt string  `json:"processed_at"`
	}
	response := make([]withdrawalResponse, len(withdrawals))
	for i, withdrawal := range withdrawals {
		response[i] = withdrawalResponse{
			Order:       withdrawal.orderId,
			Sum:         withdrawal.sum,
			ProcessedAt: withdrawal.processedAt.Format(time.RFC3339),
		}
	}

	body, err := json.Marshal(response)
	if err != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Write(body)
}

// Withdraw принимает JSON с order и sum.
// Ответы: 200 при списании, 402 при нехватке баллов, 422 при неверном номере, 400 при неположительной сумме.
func (userApi Handler) Withdraw(responseWriter http.ResponseWriter, request *http.Request) {
	var withdrawal struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}
	if err := json.NewDecoder(request.Body).Decode(&withdrawal); err != nil {
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	err := userApi.Service.Withdraw(request.Context(), withdrawal.Order, withdrawal.Sum)
	if err != nil {
		switch {
		case errors.Is(err, order.ErrIncorrectLuhn):
			responseWriter.WriteHeader(http.StatusUnprocessableEntity)
		case errors.Is(err, ErrInvalidSum):
			responseWriter.WriteHeader(http.StatusBadRequest)
		case errors.Is(err, ErrInsufficientFunds):
			responseWriter.WriteHeader(http.StatusPaymentRequired)
		default:
			responseWriter.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	responseWriter.WriteHeader(http.StatusOK)
}
