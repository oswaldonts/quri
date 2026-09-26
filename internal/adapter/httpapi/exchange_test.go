package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

const ratePath = "/periods/2026-07/rates/USD-PEN"

func TestRateEndpoints(t *testing.T) {
	h := newTestRouter(t)

	assertProblem(t, send(h, http.MethodGet, ratePath, ""), http.StatusNotFound)

	rec := send(h, http.MethodPut, ratePath, `{"rate": "3.25"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}

	rec = send(h, http.MethodGet, ratePath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got Rate
	decode(t, rec, &got)
	if got.Period != "2026-07" || got.From != "USD" || got.To != "PEN" || got.Rate != "3.2500" {
		t.Errorf("response = %+v", got)
	}
}

func TestRateEndpointsErrors(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"pair without dash", http.MethodPut, "/periods/2026-07/rates/USDPEN", `{"rate": "3.25"}`},
		{"same currency", http.MethodPut, "/periods/2026-07/rates/USD-USD", `{"rate": "3.25"}`},
		{"unsupported currency", http.MethodPut, "/periods/2026-07/rates/USD-EUR", `{"rate": "3.25"}`},
		{"rate as number", http.MethodPut, ratePath, `{"rate": 3.25}`},
		{"too many decimals", http.MethodPut, ratePath, `{"rate": "3.25671"}`},
		{"zero rate", http.MethodPut, ratePath, `{"rate": "0"}`},
		{"GET bad pair", http.MethodGet, "/periods/2026-07/rates/USDPEN", ""},
		{"GET bad period", http.MethodGet, "/periods/julio/rates/USD-PEN", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblem(t, send(newTestRouter(t), tt.method, tt.path, tt.body), http.StatusUnprocessableEntity)
		})
	}
}

const exchangeBody = `{
	"from": "USD",
	"to": "PEN",
	"amount": "1096.00",
	"description": "Casa de cambio",
	"exchanged_on": "2026-08-01"
}`

func TestRegisterExchange(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPut, ratePath, `{"rate": "3.25"}`)

	rec := send(h, http.MethodPost, "/periods/2026-07/exchanges", exchangeBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	var got Exchange
	decode(t, rec, &got)
	if got.From != (Money{"USD", "1096.00"}) {
		t.Errorf("from = %+v, want USD 1096.00", got.From)
	}
	if got.To != (Money{"PEN", "3562.00"}) {
		t.Errorf("to = %+v, want PEN 3562.00", got.To)
	}
	if got.Rate != "3.2500" {
		t.Errorf("rate = %s, want 3.2500", got.Rate)
	}
	if got.IncomeID == "" {
		t.Error("income_id is empty: the PEN income was not linked")
	}
}

func TestRegisterExchangeWithoutRate(t *testing.T) {
	rec := send(newTestRouter(t), http.MethodPost, "/periods/2026-07/exchanges", exchangeBody)
	assertProblem(t, rec, http.StatusConflict)
}

func TestRegisterExchangeErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"zero amount", strings.Replace(exchangeBody, `"1096.00"`, `"0"`, 1), http.StatusUnprocessableEntity},
		{"same currency", strings.Replace(exchangeBody, `"PEN"`, `"USD"`, 1), http.StatusUnprocessableEntity},
		{"bad date", strings.Replace(exchangeBody, `"2026-08-01"`, `"01/08/2026"`, 1), http.StatusUnprocessableEntity},
		{"malformed JSON", `{"from": `, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestRouter(t)
			send(h, http.MethodPut, ratePath, `{"rate": "3.25"}`)
			assertProblem(t, send(h, http.MethodPost, "/periods/2026-07/exchanges", tt.body), tt.wantStatus)
		})
	}
}
