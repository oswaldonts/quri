package httpapi

import (
	"net/http"
	"testing"
)

func created[T any](t *testing.T, h http.Handler, path, body string) T {
	t.Helper()
	rec := send(h, http.MethodPost, path, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST %s: status = %d (body: %s)", path, rec.Code, rec.Body)
	}
	var v T
	decode(t, rec, &v)
	return v
}

func TestEditAndDeleteIncome(t *testing.T) {
	h := newTestRouter(t)
	inc := created[Income](t, h, "/periods/2026-07/incomes", incomeBody)
	path := "/periods/2026-07/incomes/" + inc.ID

	rec := send(h, http.MethodPatch, path, `{"amount": "1350.50"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: status = %d (body: %s)", rec.Code, rec.Body)
	}
	var got Income
	decode(t, rec, &got)
	if got.Amount != "1350.50" || got.Description != "Sueldo julio" || got.ID != inc.ID {
		t.Errorf("after PATCH = %+v", got)
	}

	assertProblem(t, send(h, http.MethodPatch, path, `{"amount": "0"}`), http.StatusUnprocessableEntity)
	assertProblem(t, send(h, http.MethodPatch, "/periods/2026-08/incomes/"+inc.ID, `{"amount": "1"}`), http.StatusNotFound)

	if rec := send(h, http.MethodDelete, path, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: status = %d (body: %s)", rec.Code, rec.Body)
	}
	assertProblem(t, send(h, http.MethodDelete, path, ""), http.StatusNotFound)
}

func TestEditAndDeleteExpense(t *testing.T) {
	h := newTestRouter(t)
	e := created[Expense](t, h, "/periods/2026-07/expenses", expenseBody)
	path := "/periods/2026-07/expenses/" + e.ID

	rec := send(h, http.MethodPatch, path, `{"amount": "54.50", "tags": ["#Dulces"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH: status = %d (body: %s)", rec.Code, rec.Body)
	}
	var got Expense
	decode(t, rec, &got)
	if got.Amount != "54.50" || len(got.Tags) != 1 || got.Tags[0] != "dulces" || got.Description != "Mercado semanal" {
		t.Errorf("after PATCH = %+v", got)
	}

	if rec := send(h, http.MethodDelete, path, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: status = %d", rec.Code)
	}
	if es := listExpenses(t, h, "2026-07"); len(es) != 0 {
		t.Errorf("expenses after DELETE = %d, want 0", len(es))
	}
}

func TestExchangeEntriesAreProtectedAndDeletedTogether(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPut, ratePath, `{"rate": "3.25"}`)
	ex := created[Exchange](t, h, "/periods/2026-07/exchanges", exchangeBody)

	var exchangeExpense Expense
	for _, e := range listExpenses(t, h, "2026-07") {
		if e.ExchangeID == ex.ID {
			exchangeExpense = e
		}
	}
	// The income and expense of an exchange can't be touched on their own.
	assertProblem(t, send(h, http.MethodPatch, "/periods/2026-07/incomes/"+ex.IncomeID, `{"amount": "1"}`), http.StatusConflict)
	assertProblem(t, send(h, http.MethodDelete, "/periods/2026-07/incomes/"+ex.IncomeID, ""), http.StatusConflict)
	assertProblem(t, send(h, http.MethodDelete, "/periods/2026-07/expenses/"+exchangeExpense.ID, ""), http.StatusConflict)

	var list []Exchange
	decode(t, send(h, http.MethodGet, "/periods/2026-07/exchanges", ""), &list)
	if len(list) != 1 || list[0].ID != ex.ID {
		t.Fatalf("exchanges = %+v", list)
	}

	if rec := send(h, http.MethodDelete, "/periods/2026-07/exchanges/"+ex.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE exchange: status = %d (body: %s)", rec.Code, rec.Body)
	}
	var incomes []Income
	decode(t, send(h, http.MethodGet, "/periods/2026-07/incomes", ""), &incomes)
	if len(incomes) != 0 || len(listExpenses(t, h, "2026-07")) != 0 {
		t.Error("deleting the exchange must delete its income and expense")
	}
}

func TestDeleteBalance(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPut, "/periods/2026-07/balances/PEN", `{"amount": "10"}`)

	if rec := send(h, http.MethodDelete, "/periods/2026-07/balances/PEN", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: status = %d", rec.Code)
	}
	assertProblem(t, send(h, http.MethodDelete, "/periods/2026-07/balances/PEN", ""), http.StatusNotFound)
}
