package httpapi

import (
	"net/http"
	"testing"
)

func TestBalances(t *testing.T) {
	h := newTestRouter(t)

	for _, body := range []string{`{"amount": "400.00"}`, `{"amount": "415.50"}`} {
		rec := send(h, http.MethodPut, "/periods/2026-07/balances/PEN", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT: status = %d (body: %s)", rec.Code, rec.Body)
		}
	}
	send(h, http.MethodPut, "/periods/2026-07/balances/USD", `{"amount": "-20"}`)

	var got []Balance
	decode(t, send(h, http.MethodGet, "/periods/2026-07/balances", ""), &got)
	if len(got) != 2 || got[0].Currency != "PEN" || got[0].Amount != "415.50" || got[1].Currency != "USD" || got[1].Amount != "-20.00" {
		t.Errorf("balances = %+v, want PEN 415.50 and USD -20.00", got)
	}
}

func TestBalanceErrors(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{"unsupported currency", "/periods/2026-07/balances/EUR", `{"amount": "1"}`},
		{"bad amount", "/periods/2026-07/balances/PEN", `{"amount": "1.005"}`},
		{"bad period", "/periods/2026-7/balances/PEN", `{"amount": "1"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblem(t, send(newTestRouter(t), http.MethodPut, tt.path, tt.body), http.StatusUnprocessableEntity)
		})
	}
}
