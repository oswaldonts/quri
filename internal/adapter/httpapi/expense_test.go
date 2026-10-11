package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

const expenseBody = `{
	"category_id": "market",
	"currency": "PEN",
	"amount": "45.50",
	"description": "Mercado semanal",
	"spent_on": "2026-08-03"
}`

func listExpenses(t *testing.T, h http.Handler, period string) []Expense {
	t.Helper()
	rec := send(h, http.MethodGet, "/periods/"+period+"/expenses", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list expenses: status = %d (body: %s)", rec.Code, rec.Body)
	}
	var es []Expense
	decode(t, rec, &es)
	return es
}

func TestRegisterAndListExpenses(t *testing.T) {
	h := newTestRouter(t)

	rec := send(h, http.MethodPost, "/periods/2026-07/expenses", expenseBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	var got Expense
	decode(t, rec, &got)
	if got.CategoryID != "market" || got.Currency != "PEN" || got.Amount != "45.50" || got.SpentOn != "2026-08-03" || got.ExchangeID != "" {
		t.Errorf("response = %+v", got)
	}

	if es := listExpenses(t, h, "2026-07"); len(es) != 1 || es[0].ID != got.ID {
		t.Errorf("list = %+v, want the registered expense", es)
	}

	// The first expense fixes the period's category list.
	if pc := periodCategories(t, h, http.MethodGet, "/periods/2026-07/categories"); pc.Source != "own" {
		t.Errorf("categories source = %s, want own after the first expense", pc.Source)
	}
	// And a category with expenses can't be removed from the period.
	assertProblem(t, send(h, http.MethodDelete, "/periods/2026-07/categories/market", ""), http.StatusConflict)
}

func TestExchangeCreatesExpense(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPut, "/periods/2026-07/rates/USD-PEN", `{"rate": "3.25"}`)
	rec := send(h, http.MethodPost, "/periods/2026-07/exchanges", exchangeBody)
	var ex Exchange
	decode(t, rec, &ex)

	es := listExpenses(t, h, "2026-07")
	if len(es) != 1 {
		t.Fatalf("got %d expenses, want the exchange one", len(es))
	}
	if e := es[0]; e.CategoryID != "exchange" || e.Currency != "USD" || e.Amount != "1096.00" || e.ExchangeID != ex.ID {
		t.Errorf("exchange expense = %+v", e)
	}
}

func TestRegisterExpenseErrors(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodDelete, "/periods/2026-07/categories/travel", "")

	tests := []struct {
		name string
		body string
	}{
		{"category not used in the period", strings.Replace(expenseBody, `"market"`, `"travel"`, 1)},
		{"unknown category", strings.Replace(expenseBody, `"market"`, `"nope"`, 1)},
		{"system category", strings.Replace(expenseBody, `"market"`, `"exchange"`, 1)},
		{"zero amount", strings.Replace(expenseBody, `"45.50"`, `"0"`, 1)},
		{"unregistered currency", strings.Replace(expenseBody, `"PEN"`, `"GBP"`, 1)},
		{"bad date", strings.Replace(expenseBody, `"2026-08-03"`, `"03/08/2026"`, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblem(t, send(h, http.MethodPost, "/periods/2026-07/expenses", tt.body), http.StatusUnprocessableEntity)
		})
	}
	if es := listExpenses(t, h, "2026-07"); len(es) != 0 {
		t.Errorf("got %d expenses, want none after invalid requests", len(es))
	}
}

const receipt = `{
	"category_id": "market",
	"currency": "PEN",
	"spent_on": "2026-08-03",
	"items": [
		{"description": "Chocolate Milky", "amount": "27", "tags": ["dulces"]},
		{"description": "Pringles", "amount": "8.5", "tags": ["dulces", "snacks"]},
		{"description": "Queso", "amount": "37", "tags": ["lácteos"]},
		{"description": "Cine", "amount": "20", "category_id": "leisure"}
	]
}`

func TestRegisterBatchAndFilters(t *testing.T) {
	h := newTestRouter(t)

	rec := send(h, http.MethodPost, "/periods/2026-07/expenses/batch", receipt)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	var created []Expense
	decode(t, rec, &created)
	if len(created) != 4 || created[3].CategoryID != "leisure" || created[0].CategoryID != "market" {
		t.Fatalf("created = %+v", created)
	}

	filtered := func(query string) int {
		t.Helper()
		var es []Expense
		decode(t, send(h, http.MethodGet, "/periods/2026-07/expenses"+query, ""), &es)
		return len(es)
	}
	if n := filtered(""); n != 4 {
		t.Errorf("all = %d, want 4", n)
	}
	if n := filtered("?category_id=market"); n != 3 {
		t.Errorf("market = %d, want 3", n)
	}
	if n := filtered("?tag=dulces"); n != 2 {
		t.Errorf("dulces = %d, want 2", n)
	}
	if n := filtered("?category_id=market&tag=l%C3%A1cteos"); n != 1 {
		t.Errorf("market + lácteos = %d, want 1", n)
	}
}

func TestRegisterBatchIsAllOrNothing(t *testing.T) {
	h := newTestRouter(t)
	bad := strings.Replace(receipt, `"category_id": "leisure"`, `"category_id": "nope"`, 1)

	p := assertProblem(t, send(h, http.MethodPost, "/periods/2026-07/expenses/batch", bad), http.StatusUnprocessableEntity)
	if !strings.Contains(p.Detail, "item 4") {
		t.Errorf("detail = %q, want it to name item 4", p.Detail)
	}
	if es := listExpenses(t, h, "2026-07"); len(es) != 0 {
		t.Errorf("saved %d expenses, want none", len(es))
	}

	assertProblem(t, send(h, http.MethodPost, "/periods/2026-07/expenses/batch", `{"items": []}`), http.StatusUnprocessableEntity)
}
