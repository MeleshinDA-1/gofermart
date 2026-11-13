package order

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

// Handler обрабатывает HTTP-запросы заказов через auth.Middleware; внутренние ошибки дают 500.
type Handler struct {
	Service Service
}

// MakeOrder принимает номер заказа как текст в теле запроса.
// Возвращает 202 для нового заказа, 200 для своего повтора, 409 для чужого номера,
// 422 при неверном номере, 400 при ошибке чтения тела.
func (h *Handler) MakeOrder(responseWriter http.ResponseWriter, request *http.Request) {
	x, err := io.ReadAll(request.Body)
	if err != nil {
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}

	orderID := string(x)
	if err = h.Service.MakeOrder(request.Context(), orderID); err != nil {
		if errors.Is(err, ErrIncorrectLuhn) {
			responseWriter.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		if errors.Is(err, ErrForeignOrder) {
			responseWriter.WriteHeader(http.StatusConflict)
			return
		}

		if errors.Is(err, ErrRepeatingOrder) {
			responseWriter.WriteHeader(http.StatusOK)
			return
		}

		log.Printf("Сохранение заказа: %v", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseWriter.WriteHeader(http.StatusAccepted)
}

// GetOrders отвечает 200 со своими заказами от новых к старым: number, status, uploaded_at (RFC3339),
// ненулевое accrual для PROCESSED. При пустом списке возвращает 204.
func (h *Handler) GetOrders(responseWriter http.ResponseWriter, request *http.Request) {
	orders, err := h.Service.GetOrders(request.Context())
	if err != nil {
		log.Printf("Получение заказов: %v", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(orders) == 0 {
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}

	type orderResponse struct {
		Number     string  `json:"number"`
		Status     string  `json:"status"`
		Accrual    float64 `json:"accrual,omitempty"`
		UploadedAt string  `json:"uploaded_at"`
	}
	response := make([]orderResponse, len(orders))
	for i, order := range orders {
		response[i] = orderResponse{
			Number:     order.orderID,
			Status:     order.status.String(),
			UploadedAt: order.uploadDate.Format(time.RFC3339),
		}
		if order.status == StatusProcessed {
			response[i].Accrual = order.bonusesCredited
		}
	}

	body, err := json.Marshal(response)
	if err != nil {
		log.Printf("Формирование JSON заказов: %v", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.Write(body)
}
