package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type withdrawalDB struct {
	userQuerier
	begin func(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

func (db withdrawalDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	return db.begin(ctx, options)
}

type withdrawalTx struct {
	pgx.Tx
	lock     func(context.Context, string, ...any) pgx.Row
	insert   func(context.Context, string, ...any) (pgconn.CommandTag, error)
	commit   func(context.Context) error
	rollback func(context.Context) error
}

func (tx withdrawalTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.lock(ctx, sql, args...)
}

func (tx withdrawalTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.insert(ctx, sql, args...)
}

func (tx withdrawalTx) Commit(ctx context.Context) error { return tx.commit(ctx) }

func (tx withdrawalTx) Rollback(ctx context.Context) error { return tx.rollback(ctx) }

func TestWithdrawHTTP(t *testing.T) {
	token := validTestToken(t, "42")
	const body = `{"order":"00012345678903","sum":12.5,"userId":999}`
	for _, test := range []struct {
		name         string
		body         string
		orderID      string
		unauthorized bool
		insufficient bool
		failAt       string
		wantStatus   int
		wantSteps    string
	}{
		{name: "success", body: body, wantStatus: 200, wantSteps: "begin,lock,insert,commit,rollback"},
		{name: "maximum order length", body: `{"order":"` + strings.Repeat("0", 255) + `","sum":12.5}`, orderID: strings.Repeat("0", 255), wantStatus: 200, wantSteps: "begin,lock,insert,commit,rollback"},
		{name: "order too long", body: `{"order":"` + strings.Repeat("0", 256) + `","sum":12.5}`, wantStatus: 422},
		{name: "insufficient funds", body: body, insufficient: true, wantStatus: 402, wantSteps: "begin,lock,insert,rollback"},
		{name: "unauthorized", body: body, unauthorized: true, wantStatus: 401},
		{name: "malformed JSON", body: "{", wantStatus: 400},
		{name: "wrong checksum", body: `{"order":"12345678904","sum":12.5}`, wantStatus: 422},
		{name: "empty order", body: `{"sum":12.5}`, wantStatus: 422},
		{name: "zero sum", body: `{"order":"12345678903","sum":0}`, wantStatus: 400},
		{name: "negative sum", body: `{"order":"12345678903","sum":-1}`, wantStatus: 400},
		{name: "missing sum", body: `{"order":"12345678903"}`, wantStatus: 400},
		{name: "begin failure", body: body, failAt: "begin", wantStatus: 500, wantSteps: "begin"},
		{name: "lock failure", body: body, failAt: "lock", wantStatus: 500, wantSteps: "begin,lock,rollback"},
		{name: "insert failure", body: body, failAt: "insert", wantStatus: 500, wantSteps: "begin,lock,insert,rollback"},
		{name: "commit failure", body: body, failAt: "commit", wantStatus: 500, wantSteps: "begin,lock,insert,commit,rollback"},
	} {
		t.Run(test.name, func(t *testing.T) {
			orderID := test.orderID
			if orderID == "" {
				orderID = "00012345678903"
			}
			var steps []string
			step := func(name string) error {
				steps = append(steps, name)
				if name == test.failAt {
					return errors.New("database failure")
				}
				return nil
			}
			var transactionContext context.Context
			tx := withdrawalTx{
				lock: func(ctx context.Context, sql string, args ...any) pgx.Row {
					if ctx != transactionContext || !strings.Contains(sql, "FOR UPDATE") ||
						!reflect.DeepEqual(args, []any{int64(42)}) {
						t.Fatalf("unexpected user lock: %s, %v", sql, args)
					}
					return idRow(42, step("lock"))
				},
				insert: func(ctx context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
					if ctx != transactionContext || !reflect.DeepEqual(args, []any{int64(42), orderID, 12.5}) {
						t.Fatalf("unexpected withdrawal arguments: %v", args)
					}
					err := step("insert")
					if test.insufficient || err != nil {
						return pgconn.NewCommandTag("INSERT 0 0"), err
					}
					return pgconn.NewCommandTag("INSERT 0 1"), nil
				},
				commit: func(ctx context.Context) error {
					if ctx != transactionContext {
						t.Fatal("commit lost the request context")
					}
					return step("commit")
				},
				rollback: func(context.Context) error { return step("rollback") },
			}
			db := withdrawalDB{begin: func(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
				transactionContext = ctx
				if options.IsoLevel != pgx.ReadCommitted {
					t.Fatal("balance must be read from a fresh snapshot after locking")
				}
				return tx, step("begin")
			}}
			api := NewHandler(Service{Repository: Repository{pool: db}})
			request := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(test.body))
			if !test.unauthorized {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			testMux(api).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if response.Body.Len() != 0 {
				t.Fatalf("unexpected body: %s", response.Body.String())
			}
			if got := strings.Join(steps, ","); got != test.wantSteps {
				t.Fatalf("transaction steps = %s, want %s", got, test.wantSteps)
			}
		})
	}
}

func TestConcurrentWithdrawals(t *testing.T) {
	token := validTestToken(t, "42")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rowLock := make(chan struct{}, 1)
	rowLock <- struct{}{}
	ready := make(chan struct{}, 2)
	start := make(chan struct{})
	available := 20.0
	committed := 0
	db := withdrawalDB{begin: func(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
		if options.IsoLevel != pgx.ReadCommitted {
			t.Error("unexpected isolation level")
		}
		ready <- struct{}{}
		select {
		case <-start:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		locked := false
		pending := 0.0
		release := func() {
			if locked {
				locked = false
				rowLock <- struct{}{}
			}
		}
		return withdrawalTx{
			lock: func(ctx context.Context, sql string, args ...any) pgx.Row {
				if !strings.Contains(sql, "FOR UPDATE") || !reflect.DeepEqual(args, []any{int64(42)}) {
					return idRow(0, errors.New("expected user row lock"))
				}
				select {
				case <-rowLock:
					locked = true
					return idRow(42, nil)
				case <-ctx.Done():
					return idRow(0, ctx.Err())
				}
			},
			insert: func(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
				if !locked {
					return pgconn.CommandTag{}, errors.New("balance checked without user lock")
				}
				amount := args[2].(float64)
				if amount > available {
					return pgconn.NewCommandTag("INSERT 0 0"), nil
				}
				pending = amount
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
			commit: func(context.Context) error {
				available -= pending
				committed++
				release()
				return nil
			},
			rollback: func(context.Context) error {
				release()
				return nil
			},
		}, nil
	}}
	mux := testMux(NewHandler(Service{Repository: Repository{pool: db}}))
	orders := [2]string{"12345678903", "79927398713"}
	var statuses [2]int
	var requests sync.WaitGroup
	for i := range statuses {
		requests.Add(1)
		go func() {
			defer requests.Done()
			request := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw",
				strings.NewReader(`{"order":"`+orders[i]+`","sum":12.5}`)).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			statuses[i] = response.Code
		}()
	}
	for range 2 {
		select {
		case <-ready:
		case <-ctx.Done():
			close(start)
			requests.Wait()
			t.Fatal("requests did not start transactions")
		}
	}
	close(start)
	requests.Wait()
	if !((statuses[0] == 200 && statuses[1] == 402) || (statuses[0] == 402 && statuses[1] == 200)) {
		t.Fatalf("statuses = %v, want one 200 and one 402", statuses)
	}
	if available != 7.5 || committed != 1 {
		t.Fatalf("remaining balance = %v, committed withdrawals = %d", available, committed)
	}
}
