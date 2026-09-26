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

func periodCategories(t *testing.T, h http.Handler, method, path string) PeriodCategories {
	t.Helper()
	rec := send(h, method, path, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s: status = %d (body: %s)", method, path, rec.Code, rec.Body)
	}
	var pc PeriodCategories
	decode(t, rec, &pc)
	return pc
}

func hasCategory(pc PeriodCategories, id string) bool {
	for _, c := range pc.Categories {
		if c.ID == id {
			return true
		}
	}
	return false
}

func TestPeriodCategoriesFlow(t *testing.T) {
	h := newTestRouter(t)

	aug := periodCategories(t, h, http.MethodGet, "/periods/2026-08/categories")
	if aug.Source != "catalog" || len(aug.Categories) != 18 {
		t.Fatalf("august before any change = %s with %d categories, want catalog with 18", aug.Source, len(aug.Categories))
	}

	aug = periodCategories(t, h, http.MethodDelete, "/periods/2026-08/categories/travel")
	if aug.Source != "own" || len(aug.Categories) != 17 || hasCategory(aug, "travel") {
		t.Errorf("august after removing travel = %s with %d categories", aug.Source, len(aug.Categories))
	}

	sep := periodCategories(t, h, http.MethodGet, "/periods/2026-09/categories")
	if sep.Source != "inherited" || sep.InheritedFrom != "2026-08" || hasCategory(sep, "travel") {
		t.Errorf("september = %s from %q, want inherited from 2026-08 without travel", sep.Source, sep.InheritedFrom)
	}

	jul := periodCategories(t, h, http.MethodGet, "/periods/2026-07/categories")
	if jul.Source != "catalog" || !hasCategory(jul, "travel") {
		t.Errorf("july = %s, earlier periods must not be affected", jul.Source)
	}

	sep = periodCategories(t, h, http.MethodPut, "/periods/2026-09/categories/travel")
	if sep.Source != "own" || !hasCategory(sep, "travel") {
		t.Errorf("september after adding travel = %s", sep.Source)
	}
	if aug = periodCategories(t, h, http.MethodGet, "/periods/2026-08/categories"); hasCategory(aug, "travel") {
		t.Error("adding travel to september must not change august")
	}
}

func TestPeriodCategoryErrors(t *testing.T) {
	h := newTestRouter(t)
	send(h, http.MethodPatch, "/categories/other", `{"archived": true}`)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"remove system category", http.MethodDelete, "/periods/2026-08/categories/exchange", http.StatusConflict},
		{"add archived category", http.MethodPut, "/periods/2026-08/categories/other", http.StatusConflict},
		{"unknown category", http.MethodPut, "/periods/2026-08/categories/nope", http.StatusNotFound},
		{"bad period", http.MethodGet, "/periods/2026-8/categories", http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertProblem(t, send(h, tt.method, tt.path, ""), tt.wantStatus)
		})
	}
}
