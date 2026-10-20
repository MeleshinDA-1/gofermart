package order

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type pendingOrderRows struct {
	pgx.Rows
	ids   []string
	index int
}

func (r *pendingOrderRows) Next() bool { return r.index < len(r.ids) }
func (r *pendingOrderRows) Err() error { return nil }
func (r *pendingOrderRows) Close()     {}
func (r *pendingOrderRows) Scan(dest ...any) error {
	*dest[0].(*string) = r.ids[r.index]
	r.index++
	return nil
}

type pollingDB struct {
	ordersDB
	pending []string
	updates []Order
}

func (db *pollingDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &pendingOrderRows{ids: append([]string(nil), db.pending...)}, nil
}

type pollingBatch struct {
	pgx.BatchResults
	close func() error
}

func (b pollingBatch) Close() error { return b.close() }

func (db *pollingDB) SendBatch(_ context.Context, batch *pgx.Batch) pgx.BatchResults {
	return pollingBatch{close: func() error {
		for _, query := range batch.QueuedQueries {
			status, err := parseStatus(query.Arguments[1].(string))
			if err != nil {
				return err
			}
			update := Order{
				orderId:         query.Arguments[0].(string),
				status:          status,
				bonusesCredited: query.Arguments[2].(float64),
			}
			db.updates = append(db.updates, update)
			if status == StatusProcessed || status == StatusInvalid {
				for i, id := range db.pending {
					if id == update.orderId {
						db.pending = append(db.pending[:i], db.pending[i+1:]...)
						break
					}
				}
			}
		}
		return nil
	}}
}

func TestPollBonusesResumesAfterRateLimit(t *testing.T) {
	for _, oldStatus := range []int{http.StatusNoContent, http.StatusOK, http.StatusInternalServerError} {
		t.Run(http.StatusText(oldStatus), func(t *testing.T) {
			const oldOrder, readyOrder = "12345678903", "9278923470"
			db := &pollingDB{pending: []string{oldOrder, readyOrder}}
			var mu sync.Mutex
			var requests []string
			requestsInWindow := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				id := strings.TrimPrefix(r.URL.Path, "/api/orders/")
				requests = append(requests, id)
				requestsInWindow++
				if requestsInWindow > 1 {
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				if id == oldOrder {
					w.WriteHeader(oldStatus)
					if oldStatus == http.StatusOK {
						fmt.Fprintf(w, `{"order":%q,"status":"PROCESSING"}`, id)
					}
					return
				}
				fmt.Fprintf(w, `{"order":%q,"status":"PROCESSED","accrual":12.5}`, id)
			}))
			defer server.Close()
			service := Service{
				Repository:    Repository{pool: db},
				AccrualClient: *NewAccrualClient(server.URL, server.Client()),
			}

			for window := 0; window < 3; window++ {
				mu.Lock()
				requestsInWindow = 0
				mu.Unlock()
				pause, err := service.pollBonuses(context.Background())
				wantPause := time.Second
				if window == 2 {
					wantPause = 0
				}
				if err != nil || pause != wantPause {
					t.Fatalf("window %d: pause=%s, error=%v, want pause=%s", window, pause, err, wantPause)
				}
				if window == 0 && oldStatus == http.StatusOK && len(db.updates) != 1 {
					t.Fatal("update received before 429 was not saved")
				}
			}

			var processed []Order
			for _, update := range db.updates {
				if update.status == StatusProcessed {
					processed = append(processed, update)
				}
			}
			if len(processed) != 1 || processed[0].orderId != readyOrder || processed[0].bonusesCredited != 12.5 {
				t.Fatalf("ready order was not credited exactly once: %+v", processed)
			}
			mu.Lock()
			defer mu.Unlock()
			wantRequests := []string{oldOrder, readyOrder, readyOrder, oldOrder, oldOrder}
			if !reflect.DeepEqual(requests, wantRequests) {
				t.Fatalf("requests=%v, want %v", requests, wantRequests)
			}
		})
	}
}

func TestPollBonusesWhenRateLimitedOrderDisappears(t *testing.T) {
	const oldOrder, nextOrder = "12345678903", "9278923470"
	db := &pollingDB{pending: []string{oldOrder, nextOrder}}
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		id := strings.TrimPrefix(r.URL.Path, "/api/orders/")
		requests = append(requests, id)
		if id == oldOrder {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprintf(w, `{"order":%q,"status":"INVALID"}`, id)
	}))
	defer server.Close()
	service := Service{
		Repository:    Repository{pool: db},
		AccrualClient: *NewAccrualClient(server.URL, server.Client()),
	}
	if pause, err := service.pollBonuses(context.Background()); err != nil || pause != time.Second {
		t.Fatalf("first poll: pause=%s error=%v", pause, err)
	}
	db.pending = []string{nextOrder}
	if pause, err := service.pollBonuses(context.Background()); err != nil || pause != 0 {
		t.Fatalf("second poll: pause=%s error=%v", pause, err)
	}
	if len(db.pending) != 0 || len(db.updates) != 1 || db.updates[0].status != StatusInvalid {
		t.Fatalf("remaining order was not finalized: %+v", db.updates)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(requests, []string{oldOrder, nextOrder}) {
		t.Fatalf("unexpected requests: %v", requests)
	}
}
