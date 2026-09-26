package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/memory"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
)

// newExchangeTestRouter wires the real exchange service with in-memory repositories.
func newExchangeTestRouter() http.Handler {
	svc := exchange.NewService(
		memory.NewRateRepository(),
		memory.NewExchangeRepository(),
		memory.NewIncomeRepository(),
	)
	return NewRouter(Services{Rates: svc, Exchanges: svc})
}

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateEndpoints(t *testing.T) {
	h := newExchangeTestRouter()
	const path = "/periods/2026-07/rates/USD-PEN"

	if rec := send(h, http.MethodGet, path, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("GET before PUT: status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	rec := send(h, http.MethodPut, path, `{"rate": "3.25"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}

	rec = send(h, http.MethodGet, path, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got rateResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Period != "2026-07" || got.From != "USD" || got.To != "PEN" || got.Rate != "3.2500" {
		t.Errorf("response = %+v", got)
	}
}

func TestRateEndpointsBadRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"pair without dash", http.MethodPut, "/periods/2026-07/rates/USDPEN", `{"rate": "3.25"}`},
		{"same currency", http.MethodPut, "/periods/2026-07/rates/USD-USD", `{"rate": "3.25"}`},
		{"rate as number", http.MethodPut, "/periods/2026-07/rates/USD-PEN", `{"rate": 3.25}`},
		{"too many decimals", http.MethodPut, "/periods/2026-07/rates/USD-PEN", `{"rate": "3.25671"}`},
		{"GET bad pair", http.MethodGet, "/periods/2026-07/rates/USDPEN", ""},
		{"GET bad period", http.MethodGet, "/periods/julio/rates/USD-PEN", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(newExchangeTestRouter(), tt.method, tt.path, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body)
			}
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
	h := newExchangeTestRouter()
	send(h, http.MethodPut, "/periods/2026-07/rates/USD-PEN", `{"rate": "3.25"}`)

	rec := send(h, http.MethodPost, "/periods/2026-07/exchanges", exchangeBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}

	var got exchangeResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.From != (moneyResponse{"USD", "1096.00"}) {
		t.Errorf("from = %+v, want USD 1096.00", got.From)
	}
	if got.To != (moneyResponse{"PEN", "3562.00"}) {
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
	rec := send(newExchangeTestRouter(), http.MethodPost, "/periods/2026-07/exchanges", exchangeBody)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusConflict, rec.Body)
	}
}

func TestRegisterExchangeBadRequest(t *testing.T) {
	h := newExchangeTestRouter()
	send(h, http.MethodPut, "/periods/2026-07/rates/USD-PEN", `{"rate": "3.25"}`)

	body := strings.Replace(exchangeBody, `"1096.00"`, `"0"`, 1)
	if rec := send(h, http.MethodPost, "/periods/2026-07/exchanges", body); rec.Code != http.StatusBadRequest {
		t.Errorf("zero amount: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec := send(h, http.MethodPost, "/periods/2026-07/exchanges", `{"from": `); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON: status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
