package order

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
func (c *AccrualClient) GetOrdersInfo(ctx context.Context, orderID string) (*AccrualInfo, time.Duration, error) {
	address := strings.TrimRight(c.address, "/") + "/api/orders/" + url.PathEscape(orderID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, 0, err
	}

	var response *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		response, err = c.httpClient.Do(request)
		if err != nil {
			return nil, 0, err
		}
		if response.StatusCode < 500 || response.StatusCode >= 600 || attempt == 2 {
			break
		}
		response.Body.Close()

		timer := time.NewTimer(200 * time.Millisecond * time.Duration(1<<attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, 0, ctx.Err()
		case <-timer.C:
		}
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusNoContent:
		return nil, 0, nil
	case http.StatusTooManyRequests:
		retryAfter := response.Header.Get("Retry-After")
		seconds, err := strconv.Atoi(retryAfter)
		if err != nil {
			log.Printf("не удалось распарсить Retry-After: %q", retryAfter)
		}
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
	if result.Order != orderID {
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
