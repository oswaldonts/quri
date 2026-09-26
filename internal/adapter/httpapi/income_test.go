package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/memory"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

const validBody = `{
	"currency": "USD",
	"amount": "1330.00",
	"kind": "salary",
	"description": "Sueldo julio",
	"received_on": "2026-07-31"
}`

// newTestRouter wires the real service with the in-memory repository.
func newTestRouter() http.Handler {
	return NewRouter(income.NewService(memory.NewIncomeRepository()))
}

func post(h http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRegisterIncome(t *testing.T) {
	rec := post(newTestRouter(), "/periods/2026-07/incomes", validBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}

	var got incomeResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID == "" {
		t.Error("ID is empty")
	}
	want := incomeResponse{
		ID:          got.ID,
		Period:      "2026-07",
		SpendMonth:  "2026-08",
		Currency:    "USD",
		Amount:      "1330.00",
		Kind:        "salary",
		Description: "Sueldo julio",
		ReceivedOn:  "2026-07-31",
		CreatedAt:   got.CreatedAt,
	}
	if got != want {
		t.Errorf("response = %+v, want %+v", got, want)
	}
}

func TestRegisterIncomeBadRequest(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{"invalid period", "/periods/2026-7/incomes", validBody},
		{"unsupported currency", "/periods/2026-07/incomes", strings.Replace(validBody, `"USD"`, `"$"`, 1)},
		{"float amount", "/periods/2026-07/incomes", strings.Replace(validBody, `"1330.00"`, `1330.00`, 1)},
		{"unknown field", "/periods/2026-07/incomes", `{"currency": "USD", "amont": "10"}`},
		{"malformed JSON", "/periods/2026-07/incomes", `{"currency": `},
		{"empty body", "/periods/2026-07/incomes", ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(newTestRouter(), tt.path, tt.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body)
			}
			var got errorResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil || got.Error == "" {
				t.Errorf("want a JSON error message, got %q (decode err: %v)", rec.Body, err)
			}
		})
	}
}

// failingRegistrar simulates an unexpected failure (e.g. the database is down).
type failingRegistrar struct{}

func (failingRegistrar) Register(context.Context, income.RegisterInput) (income.Income, error) {
	return income.Income{}, errors.New("connection refused")
}

func TestRegisterIncomeInternalError(t *testing.T) {
	rec := post(NewRouter(failingRegistrar{}), "/periods/2026-07/incomes", validBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Error("internal error details must not leak to the client")
	}
}
