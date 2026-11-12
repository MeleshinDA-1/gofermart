package order

import (
	"context"
	"log"
	"slices"
	"sync"
	"time"
)

func (o *Service) pollBonuses(ctx context.Context) (time.Duration, error) {
	orderIDs, err := o.Repository.GetNotFinished(ctx)
	if err != nil {
		return 0, err
	}

	start := slices.Index(orderIDs, o.resumeOrderID)
	if start < 0 {
		start = 0
	}
	o.resumeOrderID = ""

	var updates []Order
	var retryAfter time.Duration
	pollCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	semaphore := make(chan struct{}, 4)
	var workers sync.WaitGroup
	var mu sync.Mutex
	next := 0

schedule:
	for ; next < len(orderIDs); next++ {
		select {
		case <-pollCtx.Done():
			break schedule
		case semaphore <- struct{}{}:
		}
		if pollCtx.Err() != nil {
			<-semaphore
			break
		}
		orderID := orderIDs[(start+next)%len(orderIDs)]
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-semaphore }()

			info, pause, err := o.AccrualClient.GetOrdersInfo(pollCtx, orderID)
			if err != nil {
				if pollCtx.Err() == nil {
					log.Printf("Получение бонусов для заказа %s: %v", orderID, err)
				}
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if pause > 0 {
				retryAfter = max(retryAfter, pause)
				cancel()
				return
			}
			if info != nil {
				updates = append(updates, Order{
					orderID:         orderID,
					status:          info.status,
					bonusesCredited: info.amountCredited,
				})
			}
		}()
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if retryAfter > 0 {
		if next == len(orderIDs) {
			next = 1
		}
		o.resumeOrderID = orderIDs[(start+next)%len(orderIDs)]
	}

	return retryAfter, o.Repository.BulkUpdate(ctx, updates)
}

// RunPoller опрашивает незавершённые заказы раз в 5 секунд до отмены ctx.
// После 429 ждёт Retry-After и продолжает обход по кругу.
// Блокирует горутину; допускается один запуск на Service.
func (o *Service) RunPoller(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			retryAfter, err := o.pollBonuses(ctx)
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
