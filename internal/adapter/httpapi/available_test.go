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
		{Currency: "PEN", Income: "4018.00", Spent: "1000.00", Available: "3018.00"},
		{Currency: "USD", Income: "1330.00", Spent: "1200.00", Available: "130.00"},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("available = %+v, want %+v", got, want)
	}

	var empty []Available
	decode(t, send(h, http.MethodGet, "/periods/2030-01/available", ""), &empty)
	if len(empty) != 0 {
		t.Errorf("empty period = %+v, want []", empty)
	}
	assertProblem(t, send(h, http.MethodGet, "/periods/2030-1/available", ""), http.StatusUnprocessableEntity)
}
