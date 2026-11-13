package order

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"gofermart/internal/auth"
)

type ordersListDB struct {
	ordersDB
	query func(context.Context, string, ...any) (pgx.Rows, error)
}

func (db ordersListDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return db.query(ctx, sql, args...)
}

type ordersListRows struct {
	pgx.Rows
	orders  []Order
	index   int
	scanErr error
	rowsErr error
	closed  bool
}

func (rows *ordersListRows) Next() bool { return rows.index < len(rows.orders) }

func (rows *ordersListRows) Err() error { return rows.rowsErr }

func (rows *ordersListRows) Close() { rows.closed = true }

func (rows *ordersListRows) Scan(dest ...any) error {
	if rows.scanErr != nil {
		return rows.scanErr
	}
	order := rows.orders[rows.index]
	rows.index++
	*dest[0].(*string) = order.orderID
	*dest[1].(*int64) = order.userID
	*dest[2].(*string) = order.status.String()
	*dest[3].(*float64) = order.bonusesCredited
	*dest[4].(*time.Time) = order.uploadDate
	return nil
}

func TestGetOrdersHTTP(t *testing.T) {
	token := validTestToken(t, "42")
	date := time.Date(2026, 10, 20, 10, 20, 30, 0, time.FixedZone("MSK", 3*60*60))
	orders := []Order{
		{orderID: "0009278923470", userID: 42, status: StatusProcessed, bonusesCredited: 500.5, uploadDate: date},
		{orderID: "12345678903", userID: 42, status: StatusProcessing, uploadDate: date.Add(-time.Hour)},
		{orderID: "346436439", userID: 42, status: StatusInvalid, uploadDate: date.Add(-2 * time.Hour)},
		{orderID: "2377225624", userID: 42, status: StatusNew, uploadDate: date.Add(-3 * time.Hour)},
	}
	const wantJSON = `[
		{"number":"0009278923470","status":"PROCESSED","accrual":500.5,"uploaded_at":"2026-10-20T10:20:30+03:00"},
		{"number":"12345678903","status":"PROCESSING","uploaded_at":"2026-10-20T09:20:30+03:00"},
		{"number":"346436439","status":"INVALID","uploaded_at":"2026-10-20T08:20:30+03:00"},
		{"number":"2377225624","status":"NEW","uploaded_at":"2026-10-20T07:20:30+03:00"}
	]`
	tests := []struct {
		name       string
		orders     []Order
		noAuth     bool
		queryErr   error
		scanErr    error
		rowsErr    error
		wantStatus int
	}{
		{name: "JSON contract", orders: orders, wantStatus: 200},
		{name: "empty list", wantStatus: 204},
		{name: "unauthorized", noAuth: true, wantStatus: 401},
		{name: "query failure", queryErr: errors.New("private database error"), wantStatus: 500},
		{name: "scan failure", orders: orders, scanErr: errors.New("scan failure"), wantStatus: 500},
		{name: "rows failure", orders: orders, rowsErr: errors.New("connection lost"), wantStatus: 500},
		{name: "unknown stored status", orders: []Order{{status: Status(99)}}, wantStatus: 500},
		{name: "invalid JSON number", orders: []Order{{status: StatusProcessed, bonusesCredited: math.NaN()}}, wantStatus: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := &ordersListRows{orders: test.orders, scanErr: test.scanErr, rowsErr: test.rowsErr}
			calls := 0
			db := ordersListDB{query: func(ctx context.Context, _ string, args ...any) (pgx.Rows, error) {
				calls++
				if len(args) != 1 || args[0] != int64(42) {
					t.Fatalf("expected user ID from token, got %v", args)
				}
				if test.queryErr != nil {
					return nil, test.queryErr
				}
				return rows, nil
			}}
			api := Handler{Service: Service{Repository: Repository{pool: db}}}

			mux := http.NewServeMux()
			mux.Handle("GET /api/user/orders", auth.Middleware(http.HandlerFunc(api.GetOrders), testJWTSecret))
			request := httptest.NewRequest(http.MethodGet, "/api/user/orders?userId=999", nil)
			if !test.noAuth {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.noAuth {
				if calls != 0 {
					t.Fatal("unauthorized request reached database")
				}
			} else if calls != 1 || (test.queryErr == nil && !rows.closed) {
				t.Fatalf("query calls = %d, rows closed = %v", calls, rows.closed)
			}
			if test.wantStatus != http.StatusOK {
				if response.Body.Len() != 0 {
					t.Fatalf("unexpected response body: %s", response.Body)
				}
				return
			}
			if response.Header().Get("Content-Type") != "application/json" {
				t.Fatal("missing JSON content type")
			}
			var got, want any
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected JSON: %s", response.Body)
			}
		})
	}
}
