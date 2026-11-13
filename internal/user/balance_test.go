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

	"github.com/jackc/pgx/v5"
)

func TestGetBalanceHTTP(t *testing.T) {
	authorization := "Bearer " + validTestToken(t, "42")
	for _, test := range []struct {
		name          string
		authorization string
		current       float64
		withdrawn     float64
		dbErr         error
		wantStatus    int
	}{
		{name: "balance", authorization: authorization, current: 500.5, withdrawn: 42, wantStatus: 200},
		{name: "zero balance", authorization: authorization, wantStatus: 200},
		{name: "no authorization", wantStatus: 401},
		{name: "invalid token", authorization: "Bearer invalid", wantStatus: 401},
		{name: "database failure", authorization: authorization, dbErr: errors.New("database unavailable"), wantStatus: 500},
		{name: "invalid JSON number", authorization: authorization, current: math.NaN(), wantStatus: 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			service := testService(func(_ context.Context, _ string, args ...any) pgx.Row {
				calls++
				if len(args) != 1 || args[0] != int64(42) {
					t.Fatalf("expected user ID from token, got %v", args)
				}
				return scanFunc(func(dest ...any) error {
					if test.dbErr != nil {
						return test.dbErr
					}
					*dest[0].(*float64) = test.current
					*dest[1].(*float64) = test.withdrawn
					return nil
				})
			})
			request := httptest.NewRequest(http.MethodGet, "/api/user/balance?userId=999", nil)
			request.Header.Set("Authorization", test.authorization)
			response := httptest.NewRecorder()
			testMux(NewHandler(*service)).ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			wantCalls := 1
			if test.wantStatus == http.StatusUnauthorized {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatalf("database calls = %d, want %d", calls, wantCalls)
			}
			if test.wantStatus != http.StatusOK {
				if response.Body.Len() != 0 {
					t.Fatalf("unexpected error body: %s", response.Body.String())
				}
				return
			}
			if response.Header().Get("Content-Type") != "application/json" {
				t.Fatal("missing JSON content type")
			}
			var got map[string]float64
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]float64{"current": test.current, "withdrawn": test.withdrawn}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("balance = %v, want %v", got, want)
			}
		})
	}
}
