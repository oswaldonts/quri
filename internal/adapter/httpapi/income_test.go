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
	rec := send(newTestRouter(t), http.MethodPost, "/periods/2026-07/incomes", incomeBody)

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
			rec := send(newTestRouter(t), http.MethodPost, tt.path, tt.body)
			assertProblem(t, rec, tt.wantStatus)
		})
	}
}

// failingIncomes simulates an unexpected failure (e.g. the database is down).
type failingIncomes struct{}

func (failingIncomes) Register(context.Context, income.RegisterInput) (income.Income, error) {
	return income.Income{}, errors.New("connection refused")
}

func (failingIncomes) List(context.Context, string) ([]income.Income, error) {
	return nil, errors.New("connection refused")
}

func (failingIncomes) Update(context.Context, string, string, income.UpdateInput) (income.Income, error) {
	return income.Income{}, errors.New("connection refused")
}

func (failingIncomes) Delete(context.Context, string, string) error {
	return errors.New("connection refused")
}

func TestRegisterIncomeInternalError(t *testing.T) {
	rec := send(NewRouter(Services{Incomes: failingIncomes{}}), http.MethodPost, "/periods/2026-07/incomes", incomeBody)

	p := assertProblem(t, rec, http.StatusInternalServerError)
	if strings.Contains(p.Detail, "connection refused") {
		t.Error("internal error details must not leak to the client")
	}
}

func TestContributionAndListIncomes(t *testing.T) {
	h := newTestRouter(t)

	body := `{"currency": "PEN", "amount": "108.00", "kind": "extra", "category_id": "loans", "received_on": "2026-08-01"}`
	rec := send(h, http.MethodPost, "/periods/2026-07/incomes", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	send(h, http.MethodPost, "/periods/2026-07/incomes", incomeBody)

	var list []Income
	decode(t, send(h, http.MethodGet, "/periods/2026-07/incomes", ""), &list)
	if len(list) != 2 {
		t.Fatalf("got %d incomes, want 2", len(list))
	}
	var contribution Income
	for _, in := range list {
		if in.Kind == "extra" {
			contribution = in
		}
	}
	if contribution.CategoryID != "loans" || contribution.Amount != "108.00" {
		t.Errorf("contribution = %+v", contribution)
	}

	// The category now has an entry, so it can't leave the period.
	assertProblem(t, send(h, http.MethodDelete, "/periods/2026-07/categories/loans", ""), http.StatusConflict)
}

func TestContributionErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"category on a salary", `{"currency": "USD", "amount": "1", "kind": "salary", "category_id": "loans", "received_on": "2026-07-31"}`},
		{"unknown category", `{"currency": "USD", "amount": "1", "kind": "extra", "category_id": "nope", "received_on": "2026-07-31"}`},
		{"system category", `{"currency": "USD", "amount": "1", "kind": "extra", "category_id": "exchange", "received_on": "2026-07-31"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblem(t, send(newTestRouter(t), http.MethodPost, "/periods/2026-07/incomes", tt.body), http.StatusUnprocessableEntity)
		})
	}
}
