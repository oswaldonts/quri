package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

const incomeBody = `{
	"currency": "USD",
	"amount": "1330.00",
	"kind": "salary",
	"description": "Sueldo julio",
	"received_on": "2026-07-31"
}`

func TestRegisterIncome(t *testing.T) {
	rec := send(newTestRouter(), http.MethodPost, "/periods/2026-07/incomes", incomeBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	var got Income
	decode(t, rec, &got)
	if got.ID == "" {
		t.Error("ID is empty")
	}
	want := Income{
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

func TestRegisterIncomeErrors(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
	}{
		// Rejected by Huma's schema validation before reaching the core.
		{"invalid period", "/periods/2026-7/incomes", incomeBody, http.StatusUnprocessableEntity},
		{"unsupported currency", "/periods/2026-07/incomes", strings.Replace(incomeBody, `"USD"`, `"$"`, 1), http.StatusUnprocessableEntity},
		{"amount as number", "/periods/2026-07/incomes", strings.Replace(incomeBody, `"1330.00"`, `1330.00`, 1), http.StatusUnprocessableEntity},
		{"exchange kind", "/periods/2026-07/incomes", strings.Replace(incomeBody, `"salary"`, `"exchange"`, 1), http.StatusUnprocessableEntity},
		{"unknown field", "/periods/2026-07/incomes", `{"currency": "USD", "amont": "10"}`, http.StatusUnprocessableEntity},
		// Passes the schema but is rejected by the core.
		{"zero amount", "/periods/2026-07/incomes", strings.Replace(incomeBody, `"1330.00"`, `"0"`, 1), http.StatusUnprocessableEntity},
		// The body can't be parsed at all.
		{"malformed JSON", "/periods/2026-07/incomes", `{"currency": `, http.StatusBadRequest},
		{"empty body", "/periods/2026-07/incomes", ``, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(newTestRouter(), http.MethodPost, tt.path, tt.body)
			assertProblem(t, rec, tt.wantStatus)
		})
	}
}

// failingRegistrar simulates an unexpected failure (e.g. the database is down).
type failingRegistrar struct{}

func (failingRegistrar) Register(context.Context, income.RegisterInput) (income.Income, error) {
	return income.Income{}, errors.New("connection refused")
}

func TestRegisterIncomeInternalError(t *testing.T) {
	rec := send(NewRouter(Services{Incomes: failingRegistrar{}}), http.MethodPost, "/periods/2026-07/incomes", incomeBody)

	p := assertProblem(t, rec, http.StatusInternalServerError)
	if strings.Contains(p.Detail, "connection refused") {
		t.Error("internal error details must not leak to the client")
	}
}
