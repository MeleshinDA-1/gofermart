package user

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
)

type withdrawalHistoryDB struct {
	userQuerier
	query func(context.Context, string, ...any) (pgx.Rows, error)
}

func (db withdrawalHistoryDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return db.query(ctx, sql, args...)
}

type withdrawalHistoryRows struct {
	pgx.Rows
	withdrawals []Withdrawal
	index       int
	scanErr     error
	rowsErr     error
	closed      bool
}

func (rows *withdrawalHistoryRows) Next() bool { return rows.index < len(rows.withdrawals) }

func (rows *withdrawalHistoryRows) Err() error { return rows.rowsErr }

func (rows *withdrawalHistoryRows) Close() { rows.closed = true }

func (rows *withdrawalHistoryRows) Scan(dest ...any) error {
	if rows.scanErr != nil {
		return rows.scanErr
	}
	withdrawal := rows.withdrawals[rows.index]
	rows.index++
	*dest[0].(*int64) = withdrawal.id
	*dest[1].(*int64) = withdrawal.userID
	*dest[2].(*string) = withdrawal.orderID
	*dest[3].(*float64) = withdrawal.sum
	*dest[4].(*time.Time) = withdrawal.processedAt
	return nil
}

func TestGetWithdrawalsHTTP(t *testing.T) {
	authorization := "Bearer " + validTestToken(t, "42")
	date := time.Date(2026, 10, 20, 15, 4, 5, 123456789, time.FixedZone("MSK", 3*60*60))
	withdrawals := []Withdrawal{
		{id: 2, userID: 42, orderID: "00012345678903", sum: 500.5, processedAt: date},
		{id: 1, userID: 42, orderID: "79927398713", sum: 42, processedAt: date.Add(-time.Hour)},
	}
	const wantJSON = `[
		{"order":"00012345678903","sum":500.5,"processed_at":"2026-10-20T15:04:05+03:00"},
		{"order":"79927398713","sum":42,"processed_at":"2026-10-20T14:04:05+03:00"}
	]`
	for _, test := range []struct {
		name          string
		authorization string
		withdrawals   []Withdrawal
		queryErr      error
		scanErr       error
		rowsErr       error
		wantStatus    int
	}{
		{name: "JSON contract", authorization: authorization, withdrawals: withdrawals, wantStatus: 200},
		{name: "empty history", authorization: authorization, wantStatus: 204},
		{name: "unauthorized", wantStatus: 401},
		{name: "invalid token", authorization: "Bearer invalid", wantStatus: 401},
		{name: "query failure", authorization: authorization, queryErr: errors.New("database unavailable"), wantStatus: 500},
		{name: "scan failure", authorization: authorization, withdrawals: withdrawals, scanErr: errors.New("scan failure"), wantStatus: 500},
		{name: "rows failure", authorization: authorization, withdrawals: withdrawals, rowsErr: errors.New("connection lost"), wantStatus: 500},
		{name: "invalid JSON number", authorization: authorization, withdrawals: []Withdrawal{{sum: math.NaN()}}, wantStatus: 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rows := &withdrawalHistoryRows{withdrawals: test.withdrawals, scanErr: test.scanErr, rowsErr: test.rowsErr}
			calls := 0
			db := withdrawalHistoryDB{query: func(gotCtx context.Context, _ string, args ...any) (pgx.Rows, error) {
				calls++
				if len(args) != 1 || args[0] != int64(42) {
					t.Fatalf("expected user ID from token, got %v", args)
				}
				if gotCtx.Done() != ctx.Done() {
					t.Fatal("request cancellation was not forwarded")
				}
				if test.queryErr != nil {
					return nil, test.queryErr
				}
				return rows, nil
			}}
			api := NewHandler(Service{Repository: Repository{pool: db}})
			request := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals?userId=999", nil).WithContext(ctx)
			request.Header.Set("Authorization", test.authorization)
			response := httptest.NewRecorder()
			testMux(api).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.wantStatus == http.StatusUnauthorized {
				if calls != 0 {
					t.Fatal("unauthorized request reached database")
				}
			} else if calls != 1 || (test.queryErr == nil && !rows.closed) {
				t.Fatalf("query calls = %d, rows closed = %v", calls, rows.closed)
			}
			if test.wantStatus != http.StatusOK {
				if response.Body.Len() != 0 {
					t.Fatalf("unexpected body: %s", response.Body.String())
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
				t.Fatalf("unexpected JSON: %s", response.Body.String())
			}
		})
	}
}
