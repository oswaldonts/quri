package httpapi

import (
	"net/http"
	"testing"
)

// TestAvailable reproduces the user's August end to end.
func TestAvailable(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPut, "/periods/2026-08/rates/USD-PEN", `{"rate": "3.25"}`)
	created[Income](t, h, "/periods/2026-08/incomes",
		`{"currency": "USD", "amount": "1330", "kind": "salary", "received_on": "2026-08-31"}`)
	created[Exchange](t, h, "/periods/2026-08/exchanges",
		`{"from": "USD", "to": "PEN", "amount": "1200", "exchanged_on": "2026-09-01"}`)
	created[Income](t, h, "/periods/2026-08/incomes",
		`{"currency": "PEN", "amount": "118", "kind": "extra", "category_id": "loans", "received_on": "2026-09-01"}`)
	created[Expense](t, h, "/periods/2026-08/expenses",
		`{"category_id": "health-insurance", "currency": "PEN", "amount": "1000", "spent_on": "2026-09-02"}`)

	rec := send(h, http.MethodGet, "/periods/2026-08/available", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body)
	}
	var got []Available
	decode(t, rec, &got)
	want := []Available{
		{Currency: "PEN", CarriedOver: "0.00", Income: "4018.00", Spent: "1000.00", Available: "3018.00"},
		{Currency: "USD", CarriedOver: "0.00", Income: "1330.00", Spent: "1200.00", Available: "130.00"},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("available = %+v, want %+v", got, want)
	}

	// A period before any data is empty; later periods carry August's leftover forward.
	var empty []Available
	decode(t, send(h, http.MethodGet, "/periods/2026-01/available", ""), &empty)
	if len(empty) != 0 {
		t.Errorf("period before any data = %+v, want []", empty)
	}
	assertProblem(t, send(h, http.MethodGet, "/periods/2030-1/available", ""), http.StatusUnprocessableEntity)
}

func TestCarryOver(t *testing.T) {
	h := newTestRouter(t)
	created[Income](t, h, "/periods/2026-08/incomes",
		`{"currency": "PEN", "amount": "3018", "kind": "salary", "received_on": "2026-08-31"}`)

	var sep []Available
	decode(t, send(h, http.MethodGet, "/periods/2026-09/available", ""), &sep)
	if len(sep) != 1 || sep[0].CarriedOver != "3018.00" || sep[0].CarriedFrom != "available" || sep[0].Available != "3018.00" {
		t.Fatalf("September = %+v, want 3018.00 carried from available", sep)
	}

	// Closing August with the real balance changes what September carries.
	send(h, http.MethodPut, "/periods/2026-08/balances/PEN", `{"amount": "3000"}`)
	var s PeriodCurrencySummary
	decode(t, send(h, http.MethodGet, "/periods/2026-09/summary/PEN", ""), &s)
	if s.CarriedOver != "3000.00" || s.CarriedFrom != "real" || s.Available != "3000.00" {
		t.Errorf("September summary = carried %s from %q, available %s; want 3000.00 from real", s.CarriedOver, s.CarriedFrom, s.Available)
	}

	// The first month carries nothing.
	var aug []Available
	decode(t, send(h, http.MethodGet, "/periods/2026-08/available", ""), &aug)
	if aug[0].CarriedOver != "0.00" || aug[0].CarriedFrom != "" {
		t.Errorf("August = %+v, want nothing carried", aug[0])
	}
}

// TestPlanning plans September on top of August and confirms part of it.
func TestPlanning(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPut, "/periods/2026-09/rates/USD-PEN", `{"rate": "3.25"}`)
	created[Income](t, h, "/periods/2026-08/incomes",
		`{"currency": "PEN", "amount": "3018", "kind": "salary", "received_on": "2026-08-31"}`)

	salary := created[Income](t, h, "/periods/2026-09/incomes",
		`{"currency": "USD", "amount": "1330", "kind": "salary", "received_on": "2026-09-30", "status": "planned"}`)
	ex := created[Exchange](t, h, "/periods/2026-09/exchanges",
		`{"from": "USD", "to": "PEN", "amount": "1100", "exchanged_on": "2026-10-01", "status": "planned"}`)
	insurance := created[Expense](t, h, "/periods/2026-09/expenses",
		`{"category_id": "health-insurance", "currency": "PEN", "amount": "1000", "spent_on": "2026-10-02", "status": "planned"}`)
	if salary.Status != "planned" || ex.Status != "planned" || insurance.Status != "planned" {
		t.Fatalf("statuses = %s, %s, %s; want planned", salary.Status, ex.Status, insurance.Status)
	}

	available := func(view string) map[string]string {
		t.Helper()
		var as []Available
		decode(t, send(h, http.MethodGet, "/periods/2026-09/available?view="+view, ""), &as)
		out := map[string]string{}
		for _, a := range as {
			out[a.Currency] = a.Available
		}
		return out
	}
	// Confirmed: only August's carry. Projected: + 1330 − 1100 USD; + 3575 − 1000 PEN.
	if got := available("confirmed"); got["PEN"] != "3018.00" || got["USD"] != "" {
		t.Errorf("confirmed = %v, want PEN 3018.00 and no USD", got)
	}
	if got := available("projected"); got["PEN"] != "5593.00" || got["USD"] != "230.00" {
		t.Errorf("projected = %v, want PEN 5593.00 and USD 230.00", got)
	}

	// The salary arrives and the exchange happens with 1200 USD: confirm them.
	send(h, http.MethodPatch, "/periods/2026-09/incomes/"+salary.ID, `{"status": "confirmed"}`)
	rec := send(h, http.MethodPatch, "/periods/2026-09/exchanges/"+ex.ID, `{"status": "confirmed", "amount": "1200"}`)
	var confirmed Exchange
	decode(t, rec, &confirmed)
	if confirmed.Status != "confirmed" || confirmed.To.Amount != "3900.00" {
		t.Errorf("confirmed exchange = %+v", confirmed)
	}
	if got := available("confirmed"); got["PEN"] != "6918.00" || got["USD"] != "130.00" {
		t.Errorf("confirmed after confirming = %v, want PEN 6918.00 and USD 130.00", got)
	}

	var planned []Expense
	decode(t, send(h, http.MethodGet, "/periods/2026-09/expenses?status=planned", ""), &planned)
	if len(planned) != 1 || planned[0].ID != insurance.ID {
		t.Errorf("planned expenses = %+v, want only the insurance", planned)
	}
	assertProblem(t, send(h, http.MethodGet, "/periods/2026-09/available?view=maybe", ""), http.StatusUnprocessableEntity)
}
