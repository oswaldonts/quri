package category

import (
	"context"
	"errors"
	"slices"
	"testing"
)

var (
	travel = Category{ID: "travel", Name: "Viajes", Group: GroupLifestyle}
	old    = Category{ID: "old", Name: "Antiguo", Group: GroupOther, Archived: true}
)

func names(pc PeriodCategories) []string {
	var out []string
	for _, c := range pc.Categories {
		out = append(out, c.Name)
	}
	return out
}

func TestForPeriodUsesCatalogWhenNothingConfigured(t *testing.T) {
	svc := newTestService(newFakeRepo(market, travel, exchange, old))

	got, err := svc.ForPeriod(context.Background(), "2026-08")
	if err != nil {
		t.Fatalf("ForPeriod: %v", err)
	}
	if got.Source != SourceCatalog || !slices.Equal(names(got), []string{"Cambio", "Mercado", "Viajes"}) {
		t.Errorf("got %s %v, want catalog without archived", got.Source, names(got))
	}
}

func TestDeactivateFixesThePeriodAndLaterPeriodsInherit(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newFakeRepo(market, travel, exchange))

	aug, err := svc.Deactivate(ctx, "2026-08", "travel")
	if err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	if aug.Source != SourceOwn || !slices.Equal(names(aug), []string{"Cambio", "Mercado"}) {
		t.Errorf("august = %s %v", aug.Source, names(aug))
	}

	sep, _ := svc.ForPeriod(ctx, "2026-09")
	if sep.Source != SourceInherited || sep.InheritedFrom.String() != "2026-08" || !slices.Equal(names(sep), names(aug)) {
		t.Errorf("september = %s from %s %v, want inherited from 2026-08", sep.Source, sep.InheritedFrom, names(sep))
	}

	// Earlier periods are not affected by a later configuration.
	jul, _ := svc.ForPeriod(ctx, "2026-07")
	if jul.Source != SourceCatalog || len(jul.Categories) != 3 {
		t.Errorf("july = %s %v, want the full catalog", jul.Source, names(jul))
	}
}

func TestActivate(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newFakeRepo(market, travel, exchange))

	_, _ = svc.Deactivate(ctx, "2026-08", "travel")
	got, err := svc.Activate(ctx, "2026-08", "travel")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if !slices.Contains(names(got), "Viajes") {
		t.Errorf("august = %v, want Viajes back", names(got))
	}

	// Activating an already active category is a no-op, not an error.
	again, err := svc.Activate(ctx, "2026-08", "travel")
	if err != nil || len(again.Categories) != len(got.Categories) {
		t.Errorf("second Activate = %v, %v", names(again), err)
	}
}

func TestInheritanceDropsArchived(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo(market, travel, exchange)
	svc := newTestService(repo)
	_, _ = svc.Activate(ctx, "2026-08", "travel") // fixes August with all three

	archivedTravel := travel
	archivedTravel.Archived = true
	repo.byID["travel"] = archivedTravel

	sep, _ := svc.ForPeriod(ctx, "2026-09")
	if slices.Contains(names(sep), "Viajes") {
		t.Errorf("september = %v, archived Viajes must not be inherited", names(sep))
	}
	aug, _ := svc.ForPeriod(ctx, "2026-08")
	if !slices.Contains(names(aug), "Viajes") {
		t.Errorf("august = %v, its own history must keep Viajes", names(aug))
	}
}

func TestSystemCategoryIsAlwaysIncluded(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newFakeRepo(market, travel, exchange))

	if _, err := svc.Deactivate(ctx, "2026-08", "exchange"); !errors.Is(err, ErrSystemCategory) {
		t.Fatalf("deactivate system: err = %v, want %v", err, ErrSystemCategory)
	}
	got, _ := svc.Deactivate(ctx, "2026-08", "market")
	if !slices.Contains(names(got), "Cambio") {
		t.Errorf("august = %v, Cambio must always be included", names(got))
	}
}

func TestPeriodErrors(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newFakeRepo(market, exchange, old))

	if _, err := svc.ForPeriod(ctx, "2026-8"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad period: err = %v, want %v", err, ErrInvalidInput)
	}
	if _, err := svc.Activate(ctx, "2026-08", "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown category: err = %v, want %v", err, ErrNotFound)
	}
	if _, err := svc.Activate(ctx, "2026-08", "old"); !errors.Is(err, ErrArchived) {
		t.Errorf("archived category: err = %v, want %v", err, ErrArchived)
	}
	if _, err := svc.Deactivate(ctx, "2026-8", "market"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("deactivate bad period: err = %v, want %v", err, ErrInvalidInput)
	}
}
