package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/balance"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/budget"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/summary"
)

// newTestRouter wires the real services with a fresh SQLite database, like main does.
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	categoryRepo := sqlite.NewCategoryRepository(db)
	categorySvc := category.NewService(categoryRepo, categoryRepo)
	exchangeSvc := exchange.NewService(sqlite.NewRateRepository(db), sqlite.NewExchangeRepository(db), categorySvc)
	incomeRepo := sqlite.NewIncomeRepository(db)
	expenseRepo := sqlite.NewExpenseRepository(db)
	balanceRepo := sqlite.NewBalanceRepository(db)
	budgetRepo := sqlite.NewBudgetRepository(db)
	return NewRouter(Services{
		Categories: categorySvc,
		Incomes:    income.NewService(incomeRepo, categorySvc),
		Balances:   balance.NewService(balanceRepo),
		Summaries:  summary.NewService(incomeRepo, expenseRepo, balanceRepo, categoryRepo, budgetRepo, sqlite.NewPeriodBounds(db)),
		Budgets:    budget.NewService(budgetRepo, categorySvc),
		Expenses:   expense.NewService(expenseRepo, categorySvc),
		Rates:      exchangeSvc,
		Exchanges:  exchangeSvc,
	})
}

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decode unmarshals the response body into v, failing the test on error.
func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body)
	}
}

// problem is the RFC 9457 error body returned by Huma.
type problem struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

// assertProblem checks the status code and that the body is an RFC 9457 problem.
func assertProblem(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) problem {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, wantStatus, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	var p problem
	decode(t, rec, &p)
	if p.Status != wantStatus || p.Detail == "" {
		t.Errorf("problem = %+v, want status %d and a detail", p, wantStatus)
	}
	return p
}
