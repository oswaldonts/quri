package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	rec := send(NewRouter(Services{}), http.MethodGet, "/health", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got HealthOutput
	decode(t, rec, &got.Body)
	if got.Body.Status != "ok" {
		t.Errorf("status = %q, want %q", got.Body.Status, "ok")
	}
}

func TestDocs(t *testing.T) {
	h := NewRouter(Services{})

	spec := send(h, http.MethodGet, "/openapi.yaml", "")
	if spec.Code != http.StatusOK {
		t.Fatalf("/openapi.yaml: status = %d", spec.Code)
	}
	// Every route must appear in the generated spec.
	for _, want := range []string{
		"openapi: 3.1.0",
		"/health:",
		"/categories:",
		"/categories/{id}:",
		"/periods/{period}/categories:",
		"/periods/{period}/categories/{id}:",
		"/periods/{period}/incomes:",
		"/periods/{period}/incomes/{id}:",
		"/periods/{period}/expenses/{id}:",
		"/periods/{period}/exchanges/{id}:",
		"/periods/{period}/expenses:",
		"/periods/{period}/expenses/batch:",
		"/periods/{period}/balances:",
		"/periods/{period}/balances/{currency}:",
		"/periods/{period}/summary:",
		"/periods/{period}/rates/{pair}:",
		"/periods/{period}/exchanges:",
	} {
		if !strings.Contains(spec.Body.String(), want) {
			t.Errorf("/openapi.yaml does not contain %q", want)
		}
	}

	docs := send(h, http.MethodGet, "/docs", "")
	if docs.Code != http.StatusOK || !strings.Contains(docs.Body.String(), "swagger-ui") {
		t.Errorf("/docs: status = %d, want 200 with Swagger UI", docs.Code)
	}
}
