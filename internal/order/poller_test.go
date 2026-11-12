package order

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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
				orderID:         query.Arguments[0].(string),
				status:          status,
				bonusesCredited: query.Arguments[2].(float64),
			}
			db.updates = append(db.updates, update)
			if status == StatusProcessed || status == StatusInvalid {
				for i, id := range db.pending {
					if id == update.orderID {
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
	db := &pollingDB{pending: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}}
	firstFour := make(chan struct{})
	fifthStarted := make(chan struct{})
	var requests atomic.Int32
	var recovered atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/orders/")
		if requests.Add(1) == 4 {
			close(firstFour)
		}
		if recovered.Load() {
			fmt.Fprintf(w, `{"order":%q,"status":"PROCESSED","accrual":12.5}`, id)
			return
		}
		switch id {
		case "1":
			select {
			case <-firstFour:
				fmt.Fprintf(w, `{"order":%q,"status":"PROCESSED","accrual":12.5}`, id)
			case <-r.Context().Done():
			}
		case "2":
			select {
			case <-fifthStarted:
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(http.StatusTooManyRequests)
			case <-r.Context().Done():
			}
		case "3", "4", "5":
			if id == "5" {
				close(fifthStarted)
			}
			<-r.Context().Done()
		default:
			t.Errorf("unexpected request after rate limit: %s", id)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	service := Service{Repository: Repository{pool: db}, AccrualClient: *NewAccrualClient(server.URL, server.Client())}
	if pause, err := service.pollBonuses(ctx); err != nil || pause != 2*time.Second {
		t.Fatalf("first poll: pause=%s, error=%v", pause, err)
	}
	if requests.Load() != 5 || len(db.updates) != 1 || db.updates[0].orderID != "1" {
		t.Fatalf("requests=%d, updates=%+v; want 5 requests and saved order 1", requests.Load(), db.updates)
	}
	if service.resumeOrderID != "6" {
		t.Fatalf("resume order=%q, want first unscheduled order 6", service.resumeOrderID)
	}
	recovered.Store(true)
	if pause, err := service.pollBonuses(ctx); err != nil || pause != 0 {
		t.Fatalf("resumed poll: pause=%s, error=%v", pause, err)
	}
	seen := make(map[string]bool)
	for _, update := range db.updates {
		if seen[update.orderID] || update.status != StatusProcessed || update.bonusesCredited != 12.5 {
			t.Fatalf("duplicate or invalid update: %+v", update)
		}
		seen[update.orderID] = true
	}
	if len(seen) != 9 || len(db.pending) != 0 {
		t.Fatalf("processed=%d, pending=%v; want all 9 orders processed", len(seen), db.pending)
	}
	before := requests.Load()
	if pause, err := service.pollBonuses(ctx); err != nil || pause != 0 || requests.Load() != before {
		t.Fatalf("empty poll: pause=%s, error=%v, extra requests=%d", pause, err, requests.Load()-before)
	}
}

func TestPollBonusesWhenRateLimitedOrderDisappears(t *testing.T) {
	const oldOrder, nextOrder = "12345678903", "9278923470"
	db := &pollingDB{pending: []string{oldOrder}}
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
