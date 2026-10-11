package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestCurrenciesLifecycle(t *testing.T) {
	h := newTestRouter(t)

	var list []Currency
	decode(t, send(h, http.MethodGet, "/currencies", ""), &list)
	if len(list) == 0 {
		t.Fatal("a fresh database should come with the starter currencies")
	}

	// A currency that isn't registered can't be used until it is.
	account := `{"kind":"bank","name":"Revolut","currency":"GBP","balance":"10"}`
	assertProblem(t, send(h, http.MethodPost, "/accounts", account), http.StatusUnprocessableEntity)

	var gbp Currency
	decode(t, send(h, http.MethodPost, "/currencies", `{"code":"gbp","symbol":"£"}`), &gbp)
	if gbp.Code != "GBP" || gbp.Symbol != "£" {
		t.Fatalf("created = %+v, want GBP £", gbp)
	}
	assertProblem(t, send(h, http.MethodPost, "/currencies", `{"code":"GBP"}`), http.StatusConflict)
	assertProblem(t, send(h, http.MethodPost, "/currencies", `{"code":"GB"}`), http.StatusUnprocessableEntity)

	if rec := send(h, http.MethodPost, "/accounts", account); rec.Code != http.StatusCreated {
		t.Fatalf("account in a registered currency: status = %d (body: %s)", rec.Code, rec.Body)
	}

	var renamed Currency
	decode(t, send(h, http.MethodPatch, "/currencies/GBP", `{"symbol":"GBP "}`), &renamed)
	if renamed.Symbol != "GBP" {
		t.Errorf("symbol = %q, want it trimmed", renamed.Symbol)
	}

	// With an account in it, the currency can't go away.
	assertProblem(t, send(h, http.MethodDelete, "/currencies/GBP", ""), http.StatusConflict)

	if rec := send(h, http.MethodPost, "/currencies", `{"code":"CHF"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create CHF: status = %d", rec.Code)
	}
	if rec := send(h, http.MethodDelete, "/currencies/CHF", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete unused: status = %d (body: %s)", rec.Code, rec.Body)
	}
	assertProblem(t, send(h, http.MethodDelete, "/currencies/CHF", ""), http.StatusNotFound)
	assertProblem(t, send(h, http.MethodPatch, "/currencies/CHF", `{"symbol":"x"}`), http.StatusNotFound)
	assertProblem(t, send(h, http.MethodPatch, "/currencies/GBP", `{"symbol":"`+strings.Repeat("x", 17)+`"}`), http.StatusUnprocessableEntity)
}
