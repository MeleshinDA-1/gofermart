package order

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AccrualClient запрашивает результаты расчёта у внешнего сервиса начисления.
type AccrualClient struct {
	address    string
	httpClient *http.Client
}

// AccrualInfo содержит статус обработки заказа и рассчитанное начисление.
type AccrualInfo struct {
	amountCredited float64
	status         Status
}

// GetOrdersInfo получает начисление: 204 даёт nil, 429 - паузу из Retry-After
// (60 секунд при отсутствии или неверном значении).
// REGISTERED отображается в PROCESSING, начисление учитывается только при PROCESSED.
func (c *AccrualClient) GetOrdersInfo(ctx context.Context, orderId string) (*AccrualInfo, time.Duration, error) {
	address := strings.TrimRight(c.address, "/") + "/api/orders/" + url.PathEscape(orderId)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, 0, err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusNoContent:
		return nil, 0, nil
	case http.StatusTooManyRequests:
		seconds, err := strconv.Atoi(response.Header.Get("Retry-After"))
		if err != nil || seconds <= 0 {
			seconds = 60
		}
		return nil, time.Duration(seconds) * time.Second, nil
	case http.StatusOK:
	default:
		return nil, 0, fmt.Errorf("accrual service returned HTTP %d", response.StatusCode)
	}

	var result struct {
		Order   string  `json:"order"`
		Status  string  `json:"status"`
		Accrual float64 `json:"accrual"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, 0, err
	}
	if result.Order != orderId {
		return nil, 0, fmt.Errorf("accrual service returned a different order: %s", result.Order)
	}
	if result.Status == "REGISTERED" {
		result.Status = "PROCESSING"
	}
	status, err := parseStatus(result.Status)
	if err != nil {
		return nil, 0, err
	}

	info := &AccrualInfo{status: status}
	if status == StatusProcessed {
		info.amountCredited = result.Accrual
	}
	return info, 0, nil
}

// NewAccrualClient создаёт клиент начислений с базовым URL address и заданным HTTP-клиентом.
func NewAccrualClient(address string, httpClient *http.Client) *AccrualClient {
	return &AccrualClient{address: address, httpClient: httpClient}
}
