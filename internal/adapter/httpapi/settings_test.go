package httpapi

import (
	"net/http"
	"testing"
)

func TestSettingsSpendOffset(t *testing.T) {
	h := newTestRouter(t)

	rec := send(h, http.MethodGet, "/settings", "")
	var s Settings
	decode(t, rec, &s)
	if rec.Code != http.StatusOK || s.SpendOffset != 0 {
		t.Fatalf("fresh settings = %d %+v, want offset 0", rec.Code, s)
	}

	rec = send(h, http.MethodPatch, "/settings", `{"spend_offset": 1}`)
	decode(t, rec, &s)
	if rec.Code != http.StatusOK || s.SpendOffset != 1 {
		t.Fatalf("patched settings = %d %+v, want offset 1", rec.Code, s)
	}

	rec = send(h, http.MethodGet, "/periods/2026-12/summary", "")
	var sum Summary
	decode(t, rec, &sum)
	if sum.SpendMonth != "2027-01" {
		t.Errorf("spend_month = %s, want 2027-01 with offset 1", sum.SpendMonth)
	}

	assertProblem(t, send(h, http.MethodPatch, "/settings", `{"spend_offset": 2}`), http.StatusUnprocessableEntity)
}

func TestDefaultCurrencyAndOrder(t *testing.T) {
	h := newTestRouter(t)

	rec := send(h, http.MethodPatch, "/settings", `{"default_currency": "pen"}`)
	var s Settings
	decode(t, rec, &s)
	if rec.Code != http.StatusOK || s.DefaultCurrency != "PEN" {
		t.Fatalf("set default = %d %+v", rec.Code, s)
	}
	assertProblem(t, send(h, http.MethodPatch, "/settings", `{"default_currency": "GBP"}`), http.StatusUnprocessableEntity)

	rec = send(h, http.MethodPut, "/currencies/order", `{"codes": ["USD", "PEN", "ARS", "COP", "EUR", "VES"]}`)
	var list []Currency
	decode(t, rec, &list)
	if rec.Code != http.StatusOK || len(list) != 6 || list[0].Code != "USD" || list[1].Code != "PEN" {
		t.Fatalf("reorder = %d %+v", rec.Code, list)
	}
	rec = send(h, http.MethodGet, "/currencies", "")
	decode(t, rec, &list)
	if list[0].Code != "USD" || list[1].Code != "PEN" {
		t.Errorf("list after reorder starts with %s, %s", list[0].Code, list[1].Code)
	}
	assertProblem(t, send(h, http.MethodPut, "/currencies/order", `{"codes": ["USD"]}`), http.StatusUnprocessableEntity)

	// Removing the default currency clears the setting.
	send(h, http.MethodDelete, "/currencies/PEN", "")
	rec = send(h, http.MethodGet, "/settings", "")
	decode(t, rec, &s)
	if s.DefaultCurrency != "" {
		t.Errorf("default after deleting it = %q, want empty", s.DefaultCurrency)
	}
}
