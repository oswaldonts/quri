package httpapi

import (
	"net/http"
	"testing"
)

func TestListCategories(t *testing.T) {
	rec := send(newTestRouter(t), http.MethodGet, "/categories", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got []Category
	decode(t, rec, &got)
	if len(got) != 18 {
		t.Fatalf("got %d categories, want the 18 seeded", len(got))
	}
	last := got[len(got)-1]
	if last.ID != "exchange" || !last.System || last.Group != "system" {
		t.Errorf("last = %+v, want the system category Cambio", last)
	}
}

func TestCreateAndUpdateCategory(t *testing.T) {
	h := newTestRouter(t)

	rec := send(h, http.MethodPost, "/categories", `{"name": "Mascotas", "group": "needs"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body)
	}
	var created Category
	decode(t, rec, &created)
	if created.Name != "Mascotas" || created.Group != "needs" || created.Committed || created.Archived || created.System {
		t.Errorf("created = %+v", created)
	}

	// PATCH changes only the sent fields.
	rec = send(h, http.MethodPatch, "/categories/"+created.ID, `{"committed": true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body)
	}
	var updated Category
	decode(t, rec, &updated)
	if updated.Name != "Mascotas" || !updated.Committed {
		t.Errorf("updated = %+v", updated)
	}

	// Archived categories disappear from the default list but not from include_archived.
	send(h, http.MethodPatch, "/categories/"+created.ID, `{"archived": true}`)
	var active, all []Category
	decode(t, send(h, http.MethodGet, "/categories", ""), &active)
	decode(t, send(h, http.MethodGet, "/categories?include_archived=true", ""), &all)
	if len(active) != 18 || len(all) != 19 {
		t.Errorf("active = %d, all = %d; want 18 and 19", len(active), len(all))
	}
}

func TestCategoryErrors(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"duplicated name", http.MethodPost, "/categories", `{"name": "MERCADO", "group": "needs"}`, http.StatusConflict},
		{"system group", http.MethodPost, "/categories", `{"name": "X", "group": "system"}`, http.StatusUnprocessableEntity},
		{"empty name", http.MethodPost, "/categories", `{"name": "", "group": "needs"}`, http.StatusUnprocessableEntity},
		{"blank name", http.MethodPost, "/categories", `{"name": "   ", "group": "needs"}`, http.StatusUnprocessableEntity},
		{"rename to taken name", http.MethodPatch, "/categories/market", `{"name": "Hogar"}`, http.StatusConflict},
		{"modify system category", http.MethodPatch, "/categories/exchange", `{"name": "Divisas"}`, http.StatusConflict},
		{"unknown category", http.MethodPatch, "/categories/nope", `{"name": "X"}`, http.StatusNotFound},
		{"unknown field", http.MethodPatch, "/categories/market", `{"nombre": "X"}`, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblem(t, send(newTestRouter(t), tt.method, tt.path, tt.body), tt.wantStatus)
		})
	}
}
