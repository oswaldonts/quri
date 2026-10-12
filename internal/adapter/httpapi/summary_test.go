package httpapi

import (
	"net/http"
	"testing"
)

// TestSummary registers a small month end to end and checks the computed overview.
func TestSummary(t *testing.T) {
	h := newTestRouter(t)
	post := func(path, body string) {
		t.Helper()
		if rec := send(h, http.MethodPost, path, body); rec.Code != http.StatusCreated {
			t.Fatalf("POST %s: status = %d (body: %s)", path, rec.Code, rec.Body)
		}
	}

	post("/periods/2026-07/incomes", `{"currency": "USD", "amount": "1330", "kind": "salary", "received_on": "2026-07-31"}`)
	send(h, http.MethodPut, "/periods/2026-07/rates/USD-PEN", `{"rate": "3.25"}`)
	post("/periods/2026-07/exchanges", `{"from": "USD", "to": "PEN", "amount": "1096", "exchanged_on": "2026-08-01"}`)
	post("/periods/2026-07/incomes", `{"currency": "PEN", "amount": "108", "kind": "extra", "category_id": "loans", "received_on": "2026-08-01"}`)
	post("/periods/2026-07/expenses", `{"category_id": "loans", "currency": "PEN", "amount": "462", "spent_on": "2026-08-05"}`)
	post("/periods/2026-07/expenses", `{"category_id": "market", "currency": "PEN", "amount": "27", "description": "Chocolate Milky", "tags": ["#Dulces"], "spent_on": "2026-08-03"}`)
	post("/periods/2026-07/expenses", `{"category_id": "market", "currency": "PEN", "amount": "37", "description": "Queso", "tags": ["lácteos"], "spent_on": "2026-08-03"}`)
	send(h, http.MethodPut, "/periods/2026-07/balances/PEN", `{"amount": "3180"}`)

	rec := send(h, http.MethodGet, "/periods/2026-07/summary", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body)
	}
	var s Summary
	decode(t, rec, &s)

	if s.Period != "2026-07" || s.SpendMonth != "2026-07" || len(s.Currencies) != 2 {
		t.Fatalf("summary = %+v", s)
	}
	pen, usd := s.Currencies[0], s.Currencies[1]

	// PEN: 3562 + 108 = 3670 received; 462 + 27 + 37 = 526 spent; 3144 available; real 3180.
	if pen.Currency != "PEN" || pen.Income.Total != "3670.00" || pen.Spent != "526.00" ||
		pen.Available != "3144.00" || pen.Real != "3180.00" || pen.Difference != "-36.00" {
		t.Errorf("PEN = total %s spent %s available %s real %s difference %s",
			pen.Income.Total, pen.Spent, pen.Available, pen.Real, pen.Difference)
	}
	if pen.Expenses[0].CategoryID != "loans" || pen.Expenses[0].Spent != "462.00" {
		t.Errorf("top PEN category = %+v, want Créditos with 462.00 spent", pen.Expenses[0])
	}
	if len(pen.Contributions) != 1 || pen.Contributions[0].CategoryID != "loans" || pen.Contributions[0].Amount != "108.00" {
		t.Errorf("contributions = %+v, want 108.00 to Créditos", pen.Contributions)
	}

	// The same block, alone.
	var only PeriodCurrencySummary
	decode(t, send(h, http.MethodGet, "/periods/2026-07/summary/PEN", ""), &only)
	if only.Period != "2026-07" || only.Currency != "PEN" || only.Available != pen.Available || len(only.Contributions) != 1 {
		t.Errorf("summary/PEN = %+v", only)
	}
	if len(pen.Tags) != 2 || pen.Tags[0].Tag != "lácteos" || pen.Tags[1].Tag != "dulces" {
		t.Errorf("PEN tags = %+v, want lácteos (37) then dulces (27)", pen.Tags)
	}

	// USD: the exchange is reported apart; no real balance registered.
	if usd.Currency != "USD" || usd.Exchanged != "1096.00" || usd.Available != "234.00" || usd.Real != "" || usd.Difference != "" {
		t.Errorf("USD = %+v", usd)
	}
}

func TestSummaryEmptyAndInvalid(t *testing.T) {
	h := newTestRouter(t)
	var s Summary
	decode(t, send(h, http.MethodGet, "/periods/2030-01/summary", ""), &s)
	if len(s.Currencies) != 0 {
		t.Errorf("empty period: %d currencies, want 0", len(s.Currencies))
	}
	assertProblem(t, send(h, http.MethodGet, "/periods/2030-1/summary", ""), http.StatusUnprocessableEntity)
}

func TestCurrencySummaryWithoutActivity(t *testing.T) {
	h := newTestRouter(t)
	var s PeriodCurrencySummary
	decode(t, send(h, http.MethodGet, "/periods/2030-01/summary/ARS", ""), &s)
	if s.Currency != "ARS" || s.Spent != "0.00" || s.Available != "0.00" || s.Income.Total != "0.00" ||
		len(s.Expenses) != 0 || s.Real != "" {
		t.Errorf("empty ARS summary = %+v", s)
	}
	assertProblem(t, send(h, http.MethodGet, "/periods/2030-01/summary/EURO", ""), http.StatusUnprocessableEntity)
}

func TestSummaryCountsCashSpendingLikeAnyOther(t *testing.T) {
	h := newTestRouter(t)
	var cash, bank Account
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"cash","currency":"PEN","balance":"200"}`), &cash)
	decode(t, send(h, http.MethodPost, "/accounts", `{"kind":"bank","currency":"PEN","name":"BCP","balance":"1000"}`), &bank)
	send(h, http.MethodPost, "/periods/2026-07/incomes", `{"currency": "PEN", "amount": "1000", "kind": "salary", "received_on": "2026-07-31"}`)
	send(h, http.MethodPost, "/periods/2026-07/expenses", `{"category_id": "market", "currency": "PEN", "amount": "100", "spent_on": "2026-08-03", "account_id": "`+bank.ID+`"}`)
	send(h, http.MethodPost, "/periods/2026-07/expenses", `{"category_id": "market", "currency": "PEN", "amount": "50", "spent_on": "2026-08-04", "account_id": "`+cash.ID+`"}`)

	var s Summary
	decode(t, send(h, http.MethodGet, "/periods/2026-07/summary", ""), &s)
	pen := s.Currencies[0]
	if pen.Spent != "150.00" || pen.Available != "850.00" {
		t.Errorf("spent/available = %s/%s, want 150.00/850.00", pen.Spent, pen.Available)
	}
	if len(pen.Expenses) != 1 || pen.Expenses[0].Spent != "150.00" {
		t.Errorf("category totals = %+v, want market with 150.00", pen.Expenses)
	}
}
