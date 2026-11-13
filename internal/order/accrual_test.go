package order

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestGetOrdersInfo(t *testing.T) {
	tests := []struct {
		name       string
		httpStatus int
		body       string
		retryAfter string
		wantStatus Status
		wantAmount float64
		wantPause  time.Duration
		wantNil    bool
		wantError  bool
	}{
		{name: "processed", httpStatus: 200, body: `{"order":"123","status":"PROCESSED","accrual":12.5}`, wantStatus: StatusProcessed, wantAmount: 12.5},
		{name: "no accrual", httpStatus: 200, body: `{"order":"123","status":"PROCESSED"}`, wantStatus: StatusProcessed},
		{name: "registered", httpStatus: 200, body: `{"order":"123","status":"REGISTERED"}`, wantStatus: StatusProcessing},
		{name: "processing", httpStatus: 200, body: `{"order":"123","status":"PROCESSING"}`, wantStatus: StatusProcessing},
		{name: "ignore unfinished accrual", httpStatus: 200, body: `{"order":"123","status":"PROCESSING","accrual":12.5}`, wantStatus: StatusProcessing},
		{name: "invalid", httpStatus: 200, body: `{"order":"123","status":"INVALID"}`, wantStatus: StatusInvalid},
		{name: "ignore invalid accrual", httpStatus: 200, body: `{"order":"123","status":"INVALID","accrual":12.5}`, wantStatus: StatusInvalid},
		{name: "unknown order", httpStatus: 204, wantNil: true},
		{name: "rate limit", httpStatus: 429, retryAfter: "17", wantPause: 17 * time.Second, wantNil: true},
		{name: "missing retry after", httpStatus: 429, wantPause: time.Minute, wantNil: true},
		{name: "malformed retry after", httpStatus: 429, retryAfter: "later", wantPause: time.Minute, wantNil: true},
		{name: "zero retry after", httpStatus: 429, retryAfter: "0", wantPause: time.Minute, wantNil: true},
		{name: "negative retry after", httpStatus: 429, retryAfter: "-1", wantPause: time.Minute, wantNil: true},
		{name: "server failure", httpStatus: 500, wantError: true},
		{name: "bad JSON", httpStatus: 200, body: `{`, wantError: true},
		{name: "unknown status", httpStatus: 200, body: `{"order":"123","status":"OTHER"}`, wantError: true},
		{name: "wrong order", httpStatus: 200, body: `{"order":"456","status":"PROCESSED","accrual":100}`, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/orders/123" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Retry-After", test.retryAfter)
				w.WriteHeader(test.httpStatus)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			client := AccrualClient{address: server.URL + "/", httpClient: server.Client()}
			info, pause, err := client.GetOrdersInfo(context.Background(), "123")
			if test.wantError {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil || pause != test.wantPause {
				t.Fatalf("pause = %s, error = %v", pause, err)
			}
			if test.wantNil {
				if info != nil {
					t.Fatal("unexpected update")
				}
				return
			}
			if info == nil || info.status != test.wantStatus || info.amountCredited != test.wantAmount {
				t.Fatalf("unexpected result: %+v", info)
			}
		})
	}
}

func TestBonusClientCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client := AccrualClient{address: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := client.GetOrdersInfo(ctx, "123"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestBonusPollerStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		(*Service)(nil).RunPoller(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poller did not stop")
	}
}

func TestPollBonusesWithoutPendingOrders(t *testing.T) {
	rows := &ordersListRows{}
	db := ordersListDB{query: func(context.Context, string, ...any) (pgx.Rows, error) {
		return rows, nil
	}}
	service := Service{Repository: Repository{pool: db}}
	pause, err := service.pollBonuses(context.Background())
	if err != nil || pause != 0 {
		t.Fatalf("pause = %s, error = %v", pause, err)
	}
	if !rows.closed {
		t.Fatal("database rows were not closed")
	}
}
