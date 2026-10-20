package order

import (
	"context"
	"log"
	"slices"
	"time"
)

func (o *Service) pollBonuses(ctx context.Context) (time.Duration, error) {
	orderIds, err := o.Repository.GetNotFinished(ctx)
	if err != nil {
		return 0, err
	}

	start := slices.Index(orderIds, o.resumeOrderID)
	if start < 0 {
		start = 0
	}
	o.resumeOrderID = ""

	var updates []Order
	var retryAfter time.Duration
	for i := 0; i < len(orderIds); i++ {
		orderId := orderIds[(start+i)%len(orderIds)]
		info, pause, err := o.AccrualClient.GetOrdersInfo(ctx, orderId)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			log.Printf("Получение бонусов для заказа %s: %v", orderId, err)
			continue
		}
		if pause > 0 {
			retryAfter = pause
			o.resumeOrderID = orderId
			break
		}
		if info == nil {
			continue
		}
		updates = append(updates, Order{
			orderId:         orderId,
			status:          info.status,
			bonusesCredited: info.amountCredited,
		})
	}

	return retryAfter, o.Repository.BulkUpdate(ctx, updates)
}

// RunPoller опрашивает незавершённые заказы раз в 5 секунд до отмены ctx.
// После 429 ждёт Retry-After и продолжает обход по кругу с прерванного заказа.
// Блокирует горутину; допускается один запуск на Service.
func (service *Service) RunPoller(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			retryAfter, err := service.pollBonuses(ctx)
			if err != nil {
				log.Printf("Обновление бонусов: %v", err)
			}
			if retryAfter == 0 {
				retryAfter = 5 * time.Second
			}
			ticker.Reset(retryAfter)
		}
	}
}
