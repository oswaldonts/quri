package httpapi

import (
	"net/http"
	"testing"
)

func TestBudgetsInSummary(t *testing.T) {
	h := newTestRouter(t)

	if rec := send(h, http.MethodPut, "/periods/2026-08/budgets/market/PEN", `{"amount": "80"}`); rec.Code != http.StatusOK {
		t.Fatalf("PUT budget: status = %d (body: %s)", rec.Code, rec.Body)
	}
	send(h, http.MethodPut, "/periods/2026-08/budgets/leisure/PEN", `{"amount": "30"}`)
	created[Expense](t, h, "/periods/2026-08/expenses",
		`{"category_id": "market", "currency": "PEN", "amount": "97.50", "spent_on": "2026-08-10"}`)

	var budgets []Budget
	decode(t, send(h, http.MethodGet, "/periods/2026-08/budgets", ""), &budgets)
	if len(budgets) != 2 {
		t.Fatalf("budgets = %+v, want 2", budgets)
	}

	var s Summary
	decode(t, send(h, http.MethodGet, "/periods/2026-08/summary", ""), &s)
	pen := s.Currencies[0]
	if pen.Budgeted != "110.00" {
		t.Errorf("budgeted = %q, want 110.00", pen.Budgeted)
	}
	byID := map[string]ExpenseTotal{}
	for _, ct := range pen.Expenses {
		byID[ct.CategoryID] = ct
	}
	if m := byID["market"]; m.Budget != "80.00" || m.Remaining != "-17.50" {
		t.Errorf("Mercado = budget %q remaining %q, want 80.00 and -17.50", m.Budget, m.Remaining)
	}
	if l := byID["leisure"]; l.Spent != "0.00" || l.Remaining != "30.00" {
		t.Errorf("Ocio = spent %q remaining %q, want 0.00 and 30.00", l.Spent, l.Remaining)
	}

	// A budgeted category can't leave the period; the budget can be deleted.
	assertProblem(t, send(h, http.MethodDelete, "/periods/2026-08/categories/leisure", ""), http.StatusConflict)
	if rec := send(h, http.MethodDelete, "/periods/2026-08/budgets/leisure/PEN", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE budget: status = %d", rec.Code)
	}
}

func TestBudgetErrors(t *testing.T) {
	h := newTestRouter(t)
	assertProblem(t, send(h, http.MethodPut, "/periods/2026-08/budgets/exchange/USD", `{"amount": "1"}`), http.StatusUnprocessableEntity)
	assertProblem(t, send(h, http.MethodPut, "/periods/2026-08/budgets/nope/USD", `{"amount": "1"}`), http.StatusUnprocessableEntity)
	assertProblem(t, send(h, http.MethodPut, "/periods/2026-08/budgets/market/USD", `{"amount": "0"}`), http.StatusUnprocessableEntity)
	assertProblem(t, send(h, http.MethodDelete, "/periods/2026-08/budgets/market/USD", ""), http.StatusNotFound)
}
