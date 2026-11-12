package order

import (
	"errors"
	"time"
)

// Order хранит номер заказа, владельца, состояние начисления и время загрузки.
type Order struct {
	orderID         string
	userID          int64
	status          Status
	bonusesCredited float64
	uploadDate      time.Time
}

// Status обозначает состояние обработки заказа системой начисления.
type Status int

const (
	// StatusNew означает, что заказ принят и ожидает обработки.
	StatusNew Status = iota
	// StatusProcessing означает, что система начисления обрабатывает заказ.
	StatusProcessing
	// StatusInvalid означает, что система начисления отклонила заказ.
	StatusInvalid
	// StatusProcessed означает, что расчёт начисления завершён.
	StatusProcessed
)

// String возвращает статус в формате БД и API или пустую строку для неизвестного значения.
func (status Status) String() string {
	switch status {
	case StatusNew:
		return "NEW"
	case StatusProcessing:
		return "PROCESSING"
	case StatusInvalid:
		return "INVALID"
	case StatusProcessed:
		return "PROCESSED"
	}
	return ""
}

func parseStatus(status string) (Status, error) {
	switch status {
	case "NEW":
		return StatusNew, nil
	case "PROCESSING":
		return StatusProcessing, nil
	case "INVALID":
		return StatusInvalid, nil
	case "PROCESSED":
		return StatusProcessed, nil
	}
	return 0, errors.New("unknown order status: " + status)
}
