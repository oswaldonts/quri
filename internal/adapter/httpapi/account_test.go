package httpapi

import (
	"net/http"
	"testing"
)

func listAccounts(t *testing.T, h http.Handler) []Account {
	t.Helper()
	rec := send(h, http.MethodGet, "/accounts", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list accounts: status = %d (body: %s)", rec.Code, rec.Body)
	}
	var as []Account
	decode(t, rec, &as)
	return as
}

func TestAccountsLifecycle(t *testing.T) {
	h := newTestRouter(t)

	var cash, bcp Account
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"cash","currency":"PEN","balance":"200"}`), &cash)
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP","currency":"PEN","balance":"1000.50"}`), &bcp)
	send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"Pichincha","currency":"PEN","balance":"270.25"}`)

	if cash.Name != "Efectivo" || cash.Balance != "200.00" {
		t.Errorf("cash = %+v, want named Efectivo with 200.00", cash)
	}
	as := listAccounts(t, h)
	if len(as) != 3 || as[0].Kind != "cash" || as[1].Name != "BCP" || as[2].Name != "Pichincha" {
		t.Fatalf("accounts = %+v, want cash, BCP, Pichincha", as)
	}

	var renamed Account
	decode(t, send(h, http.MethodPatch, "/accounts/"+bcp.ID, `{"name":"BCP ahorros","balance":"900"}`), &renamed)
	if renamed.Name != "BCP ahorros" || renamed.Balance != "900.00" {
		t.Errorf("updated = %+v", renamed)
	}

	if rec := send(h, http.MethodDelete, "/accounts/"+bcp.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status = %d (body: %s)", rec.Code, rec.Body)
	}
	assertProblem(t, send(h, http.MethodDelete, "/accounts/"+bcp.ID, ""), http.StatusNotFound)
	assertProblem(t, send(h, http.MethodPatch, "/accounts/"+bcp.ID, `{"balance":"1"}`), http.StatusNotFound)
}

func TestAccountErrors(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPost, "/accounts", `{"kind":"cash","currency":"PEN","balance":"10"}`)
	send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP","currency":"PEN","balance":"10"}`)

	tests := []struct {
		name string
		body string
		want int
	}{
		{"second cash in the currency", `{"kind":"cash","currency":"PEN","balance":"5"}`, http.StatusConflict},
		{"cash in another currency", `{"kind":"cash","currency":"USD","balance":"5"}`, http.StatusCreated},
		{"repeated bank name", `{"kind":"bank","name":"bcp","currency":"PEN","balance":"5"}`, http.StatusConflict},
		{"same name in another currency", `{"kind":"bank","name":"BCP","currency":"USD","balance":"5"}`, http.StatusCreated},
		{"bank without name", `{"kind":"bank","currency":"PEN","balance":"5"}`, http.StatusUnprocessableEntity},
		{"unknown kind", `{"kind":"card","currency":"PEN","balance":"5"}`, http.StatusUnprocessableEntity},
		{"bad amount", `{"kind":"bank","name":"X","currency":"PEN","balance":"1.005"}`, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := send(h, http.MethodPost, "/accounts", tt.body); rec.Code != tt.want {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestExpensesLowerTheBalanceOfTheirAccount(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPost, "/periods/2026-07/incomes", `{"kind":"salary","currency":"PEN","amount":"1000","received_on":"2026-07-25"}`)
	var cash, bcp Account
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"cash","currency":"PEN","balance":"200"}`), &cash)
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP","currency":"PEN","balance":"500"}`), &bcp)

	register := func(body string) Expense {
		t.Helper()
		rec := send(h, http.MethodPost, "/periods/2026-07/expenses", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("register %s: status = %d (body: %s)", body, rec.Code, rec.Body)
		}
		var e Expense
		decode(t, rec, &e)
		return e
	}
	cashExpense := register(`{"category_id":"market","currency":"PEN","amount":"30.50","spent_on":"2026-08-03","account_id":"` + cash.ID + `"}`)
	if cashExpense.AccountID != cash.ID {
		t.Errorf("expense = %+v, want charged to the cash", cashExpense)
	}
	bankExpense := register(`{"category_id":"market","currency":"PEN","amount":"100","spent_on":"2026-08-04","account_id":"` + bcp.ID + `"}`)
	if bankExpense.AccountID != bcp.ID {
		t.Errorf("expense = %+v, want charged to BCP", bankExpense)
	}
	register(`{"category_id":"market","currency":"PEN","amount":"7","spent_on":"2026-08-04"}`)
	// A planned expense hasn't happened: it doesn't lower the balance.
	register(`{"category_id":"market","currency":"PEN","amount":"50","spent_on":"2026-08-05","account_id":"` + bcp.ID + `","status":"planned"}`)

	balances := func() map[string]string {
		out := map[string]string{}
		for _, a := range listAccounts(t, h) {
			out[a.Name] = a.Balance
		}
		return out
	}
	if got := balances(); got["Efectivo"] != "169.50" || got["BCP"] != "400.00" {
		t.Errorf("balances = %v, want Efectivo 169.50 and BCP 400.00", got)
	}

	// Every expense counts as spent and lowers the available money, whichever account paid it.
	var s Summary
	decode(t, send(h, http.MethodGet, "/periods/2026-07/summary", ""), &s)
	if len(s.Currencies) != 1 || s.Currencies[0].Spent != "137.50" || s.Currencies[0].Available != "862.50" {
		t.Errorf("summary = %+v, want spent 137.50, available 862.50", s.Currencies)
	}

	// Typing the balance again restarts the count.
	decode(t, send(h, http.MethodPatch, "/accounts/"+bcp.ID, `{"balance":"300"}`), &bcp)
	if bcp.Balance != "300.00" {
		t.Errorf("BCP after recount = %+v, want 300.00", bcp)
	}

	// Moving an expense to another account, or off every account, moves the charge.
	send(h, http.MethodPatch, "/periods/2026-07/expenses/"+cashExpense.ID, `{"account_id":""}`)
	decode(t, send(h, http.MethodGet, "/periods/2026-07/summary", ""), &s)
	if s.Currencies[0].Spent != "137.50" {
		t.Errorf("spent after taking it off the cash = %s, want 137.50", s.Currencies[0].Spent)
	}
	if got := balances(); got["Efectivo"] != "200.00" {
		t.Errorf("Efectivo = %s, want 200.00 back", got["Efectivo"])
	}
}

func TestExpenseAccountErrors(t *testing.T) {
	h := newTestRouter(t)
	var usd Account
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP","currency":"USD","balance":"500"}`), &usd)

	tests := []struct {
		name string
		body string
		want int
	}{
		{"unknown account", `{"category_id":"market","currency":"PEN","amount":"1","spent_on":"2026-08-03","account_id":"nope"}`, http.StatusUnprocessableEntity},
		{"account in another currency", `{"category_id":"market","currency":"PEN","amount":"1","spent_on":"2026-08-03","account_id":"` + usd.ID + `"}`, http.StatusUnprocessableEntity},
		{"account in the same currency", `{"category_id":"market","currency":"USD","amount":"1","spent_on":"2026-08-03","account_id":"` + usd.ID + `"}`, http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := send(h, http.MethodPost, "/periods/2026-07/expenses", tt.body); rec.Code != tt.want {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.want, rec.Body)
			}
		})
	}

	// The account now has an expense: deleting it would orphan the charge.
	if rec := send(h, http.MethodDelete, "/accounts/"+usd.ID, ""); rec.Code != http.StatusConflict {
		t.Errorf("delete account in use: status = %d, want 409", rec.Code)
	}
}

func TestDisabledExpenseStopsCounting(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPost, "/periods/2026-07/incomes", `{"kind":"salary","currency":"PEN","amount":"1000","received_on":"2026-07-25"}`)
	var bcp Account
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP","currency":"PEN","balance":"500"}`), &bcp)
	var e Expense
	decode(t, send(h, http.MethodPost, "/periods/2026-07/expenses",
		`{"category_id":"market","currency":"PEN","amount":"100","spent_on":"2026-08-04","account_id":"`+bcp.ID+`"}`), &e)
	if e.Disabled {
		t.Fatalf("new expense = %+v, want enabled", e)
	}

	check := func(label, spent, balance string) {
		t.Helper()
		var s Summary
		decode(t, send(h, http.MethodGet, "/periods/2026-07/summary", ""), &s)
		if got := s.Currencies[0].Spent; got != spent {
			t.Errorf("%s: spent = %s, want %s", label, got, spent)
		}
		if got := listAccounts(t, h)[0].Balance; got != balance {
			t.Errorf("%s: BCP = %s, want %s", label, got, balance)
		}
	}
	check("enabled", "100.00", "400.00")

	decode(t, send(h, http.MethodPatch, "/periods/2026-07/expenses/"+e.ID, `{"disabled":true}`), &e)
	if !e.Disabled {
		t.Errorf("after disabling = %+v", e)
	}
	check("disabled", "0.00", "500.00")

	decode(t, send(h, http.MethodPatch, "/periods/2026-07/expenses/"+e.ID, `{"amount":"80"}`), &e)
	if !e.Disabled {
		t.Errorf("editing the amount re-enabled the expense: %+v", e)
	}

	decode(t, send(h, http.MethodPatch, "/periods/2026-07/expenses/"+e.ID, `{"disabled":false}`), &e)
	check("enabled again", "80.00", "420.00")
}
