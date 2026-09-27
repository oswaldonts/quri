package httpapi

import (
	"net/http"
	"testing"
)

func TestTrends(t *testing.T) {
	h := newTestRouter(t)
	created[Expense](t, h, "/periods/2026-06/expenses",
		`{"category_id": "market", "currency": "PEN", "amount": "100", "tags": ["dulces"], "spent_on": "2026-07-03"}`)
	created[Expense](t, h, "/periods/2026-08/expenses",
		`{"category_id": "market", "currency": "PEN", "amount": "50", "spent_on": "2026-09-03"}`)
	created[Expense](t, h, "/periods/2026-08/expenses",
		`{"category_id": "leisure", "currency": "PEN", "amount": "20", "spent_on": "2026-09-05"}`)

	rec := send(h, http.MethodGet, "/trends?from=2026-06&to=2026-08&currency=PEN", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body)
	}
	var tr Trends
	decode(t, rec, &tr)

	if tr.Months != 3 || len(tr.Periods) != 3 || tr.Periods[1].Spent != "0.00" {
		t.Errorf("periods = %+v", tr.Periods)
	}
	if len(tr.Categories) != 2 || tr.Categories[0].ID != "market" || tr.Categories[0].Total != "150.00" ||
		tr.Categories[0].Average != "50.00" || len(tr.Categories[0].Series) != 3 {
		t.Errorf("categories = %+v", tr.Categories)
	}
	if len(tr.Tags) != 1 || tr.Tags[0].Tag != "dulces" {
		t.Errorf("tags = %+v", tr.Tags)
	}
}

func TestTrendsErrors(t *testing.T) {
	h := newTestRouter(t)
	for _, query := range []string{
		"?from=2026-08&to=2026-06&currency=PEN", // from after to
		"?from=2024-01&to=2026-06&currency=PEN", // more than 24 months
		"?from=2026-06&to=2026-08&currency=EUR", // unsupported currency
		"?from=2026-06&to=2026-08",              // currency missing
	} {
		assertProblem(t, send(h, http.MethodGet, "/trends"+query, ""), http.StatusUnprocessableEntity)
	}
}
