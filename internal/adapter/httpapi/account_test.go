package httpapi

import (
	"net/http"
	"net/http/httptest"
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

func TestRecurringExpense(t *testing.T) {
	h := newTestRouter(t)
	var first Expense
	decode(t, send(h, http.MethodPost, "/periods/2026-07/expenses",
		`{"category_id":"market","currency":"PEN","amount":"90","description":"Seguro","spent_on":"2026-07-10","occurrences":3}`), &first)
	if first.SeriesID == "" || first.SeriesNo != 1 || first.SeriesTotal != 3 {
		t.Fatalf("first = %+v", first)
	}

	list := func(p string) []Expense {
		var es []Expense
		decode(t, send(h, http.MethodGet, "/periods/"+p+"/expenses", ""), &es)
		return es
	}
	next := list("2026-09")
	if len(next) != 1 || next[0].SeriesNo != 3 || next[0].Status != "planned" || next[0].SpentOn != "2026-09-10" {
		t.Fatalf("2026-09 = %+v", next)
	}
	if got := list("2026-10"); len(got) != 0 {
		t.Errorf("2026-10 = %+v, want none", got)
	}

	rec := send(h, http.MethodPost, "/periods/2026-07/expenses/"+first.ID+"/repeat", `{"occurrences":5}`)
	assertProblem(t, rec, http.StatusUnprocessableEntity)

	var single Expense
	decode(t, send(h, http.MethodPost, "/periods/2026-07/expenses",
		`{"category_id":"market","currency":"PEN","amount":"30","spent_on":"2026-07-12"}`), &single)
	rec = send(h, http.MethodPost, "/periods/2026-07/expenses/"+single.ID+"/repeat", `{"occurrences":2,"first_no":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("repeat status = %d (body: %s)", rec.Code, rec.Body)
	}
	decode(t, rec, &single)
	if single.SeriesNo != 1 || single.SeriesTotal != 2 || len(list("2026-08")) != 2 {
		t.Errorf("after repeat: %+v, 2026-08 has %d", single, len(list("2026-08")))
	}

	august := list("2026-08")
	var seriesAug Expense
	for _, e := range august {
		if e.SeriesID == first.SeriesID {
			seriesAug = e
		}
	}
	if rec := send(h, http.MethodDelete, "/periods/2026-08/expenses/"+seriesAug.ID+"?following=true", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete following status = %d", rec.Code)
	}
	if len(list("2026-07")) != 2 || len(list("2026-08")) != 1 || len(list("2026-09")) != 0 {
		t.Errorf("after delete following: %d / %d / %d", len(list("2026-07")), len(list("2026-08")), len(list("2026-09")))
	}
}

func TestTransfersMoveMoneyBetweenAccounts(t *testing.T) {
	h := newTestRouter(t)
	var bcp, ibk, usd Account
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP","currency":"PEN","balance":"500"}`), &bcp)
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"Interbank","currency":"PEN","balance":"100"}`), &ibk)
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP dólares","currency":"USD","balance":"10"}`), &usd)

	balances := func() map[string]string {
		out := map[string]string{}
		for _, a := range listAccounts(t, h) {
			out[a.Name] = a.Balance
		}
		return out
	}
	transfer := func(from, to, amount string) *httptest.ResponseRecorder {
		return send(h, http.MethodPost, "/transfers",
			`{"from_account_id":"`+from+`","to_account_id":"`+to+`","amount":"`+amount+`","transferred_on":"2026-08-03"}`)
	}

	rec := transfer(bcp.ID, ibk.ID, "150.25")
	if rec.Code != http.StatusCreated {
		t.Fatalf("transfer: status = %d (body: %s)", rec.Code, rec.Body)
	}
	var tr Transfer
	decode(t, rec, &tr)
	if got := balances(); got["BCP"] != "349.75" || got["Interbank"] != "250.25" {
		t.Errorf("balances = %v, want BCP 349.75 and Interbank 250.25", got)
	}

	// No summary counts a transfer.
	var s Summary
	decode(t, send(h, http.MethodGet, "/periods/2026-08/summary", ""), &s)
	for _, c := range s.Currencies {
		if c.Spent != "0.00" {
			t.Errorf("summary spent = %s, want 0.00", c.Spent)
		}
	}

	assertProblem(t, transfer(bcp.ID, bcp.ID, "10"), http.StatusUnprocessableEntity)
	assertProblem(t, transfer(bcp.ID, usd.ID, "10"), http.StatusUnprocessableEntity)
	assertProblem(t, transfer(bcp.ID, "nope", "10"), http.StatusUnprocessableEntity)
	assertProblem(t, transfer(bcp.ID, ibk.ID, "0"), http.StatusUnprocessableEntity)

	// An account with transfers can't be deleted.
	assertProblem(t, send(h, http.MethodDelete, "/accounts/"+ibk.ID, ""), http.StatusConflict)

	var list []Transfer
	decode(t, send(h, http.MethodGet, "/transfers", ""), &list)
	if len(list) != 1 || list[0].ID != tr.ID || list[0].Amount.Amount != "150.25" {
		t.Errorf("transfers = %+v, want the one made", list)
	}

	if rec := send(h, http.MethodDelete, "/transfers/"+tr.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete transfer: status = %d", rec.Code)
	}
	if got := balances(); got["BCP"] != "500.00" || got["Interbank"] != "100.00" {
		t.Errorf("balances after deleting = %v, want the originals back", got)
	}
	assertProblem(t, send(h, http.MethodDelete, "/transfers/"+tr.ID, ""), http.StatusNotFound)
}

func TestExchangesMoveMoneyBetweenAccounts(t *testing.T) {
	h := newTestRouter(t)
	var usd, pen, penOther Account
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP dólares","currency":"USD","balance":"1000"}`), &usd)
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"BCP soles","currency":"PEN","balance":"50"}`), &pen)
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","name":"Interbank","currency":"PEN","balance":"0"}`), &penOther)
	send(h, http.MethodPut, ratePath, `{"rate": "3.25"}`)

	balances := func() map[string]string {
		out := map[string]string{}
		for _, a := range listAccounts(t, h) {
			out[a.Name] = a.Balance
		}
		return out
	}
	register := func(body string) *httptest.ResponseRecorder {
		return send(h, http.MethodPost, "/periods/2026-07/exchanges", body)
	}

	// Each side must use its own currency.
	assertProblem(t, register(`{"from":"USD","to":"PEN","amount":"100","exchanged_on":"2026-07-10","from_account_id":"`+pen.ID+`"}`), http.StatusUnprocessableEntity)
	assertProblem(t, register(`{"from":"USD","to":"PEN","amount":"100","exchanged_on":"2026-07-10","to_account_id":"`+usd.ID+`"}`), http.StatusUnprocessableEntity)

	rec := register(`{"from":"USD","to":"PEN","amount":"100","exchanged_on":"2026-07-10","from_account_id":"` + usd.ID + `","to_account_id":"` + pen.ID + `"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("exchange: status = %d (body: %s)", rec.Code, rec.Body)
	}
	var ex Exchange
	decode(t, rec, &ex)
	if ex.FromAccountID != usd.ID || ex.ToAccountID != pen.ID {
		t.Errorf("exchange = %+v, want both accounts", ex)
	}
	if got := balances(); got["BCP dólares"] != "900.00" || got["BCP soles"] != "375.00" {
		t.Errorf("balances = %v, want 900.00 USD and 375.00 PEN", got)
	}

	// Editing the amount and the target account moves the money along.
	rec = send(h, http.MethodPatch, "/periods/2026-07/exchanges/"+ex.ID, `{"amount":"200","to_account_id":"`+penOther.ID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: status = %d (body: %s)", rec.Code, rec.Body)
	}
	if got := balances(); got["BCP dólares"] != "800.00" || got["BCP soles"] != "50.00" || got["Interbank"] != "650.00" {
		t.Errorf("balances after editing = %v", got)
	}

	// A planned exchange hasn't happened yet.
	send(h, http.MethodPatch, "/periods/2026-07/exchanges/"+ex.ID, `{"status":"planned"}`)
	if got := balances(); got["BCP dólares"] != "1000.00" || got["Interbank"] != "0.00" {
		t.Errorf("balances while planned = %v, want the originals", got)
	}
	send(h, http.MethodPatch, "/periods/2026-07/exchanges/"+ex.ID, `{"status":"confirmed"}`)

	// An account an exchange refers to can't be deleted; deleting the exchange frees it.
	assertProblem(t, send(h, http.MethodDelete, "/accounts/"+usd.ID, ""), http.StatusConflict)
	if rec := send(h, http.MethodDelete, "/periods/2026-07/exchanges/"+ex.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete exchange: status = %d", rec.Code)
	}
	if got := balances(); got["BCP dólares"] != "1000.00" || got["Interbank"] != "0.00" {
		t.Errorf("balances after deleting = %v, want the originals", got)
	}
}
