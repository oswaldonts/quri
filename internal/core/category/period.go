package category

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// ErrArchived means an archived category can't be activated in a period.
var ErrArchived = errors.New("category is archived")

// PeriodRepository stores which categories each period uses.
type PeriodRepository interface {
	// ListByPeriod returns the categories configured for the period (empty if it has none).
	ListByPeriod(ctx context.Context, p period.Period) ([]Category, error)
	// LatestConfiguredBefore returns the most recent period before p that has its own
	// categories; ok is false if there is none.
	LatestConfiguredBefore(ctx context.Context, p period.Period) (prev period.Period, ok bool, err error)
	// ReplacePeriod sets the categories of the period to exactly ids, atomically.
	ReplacePeriod(ctx context.Context, p period.Period, ids []string) error
}

// Source tells where the categories of a period come from.
type Source string

const (
	// SourceOwn: the period has its own configuration.
	SourceOwn Source = "own"
	// SourceInherited: copied from the latest earlier configured period.
	SourceInherited Source = "inherited"
	// SourceCatalog: no earlier period is configured, so all active catalog categories apply.
	SourceCatalog Source = "catalog"
)

// PeriodCategories are the categories a period uses and where they come from.
type PeriodCategories struct {
	Period        period.Period
	Source        Source
	InheritedFrom period.Period // set only when Source is SourceInherited
	Categories    []Category
}

// ForPeriod returns the categories of a period, resolving inheritance.
func (s *Service) ForPeriod(ctx context.Context, periodStr string) (PeriodCategories, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return PeriodCategories{}, invalid(err)
	}
	return s.resolve(ctx, p)
}

// Activate adds a category to a period. The period keeps its own list from then on.
func (s *Service) Activate(ctx context.Context, periodStr, id string) (PeriodCategories, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return PeriodCategories{}, invalid(err)
	}
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return PeriodCategories{}, err
	}
	if c.Archived {
		return PeriodCategories{}, fmt.Errorf("%w: %s (restore it first)", ErrArchived, c.Name)
	}

	current, err := s.resolve(ctx, p)
	if err != nil {
		return PeriodCategories{}, err
	}
	ids := idsOf(current.Categories)
	if !slices.Contains(ids, id) {
		ids = append(ids, id)
	}
	return s.replace(ctx, p, ids)
}

// Deactivate removes a category from a period (the category itself is untouched).
// The period keeps its own list from then on. System categories can't be removed.
func (s *Service) Deactivate(ctx context.Context, periodStr, id string) (PeriodCategories, error) {
	p, err := period.Parse(periodStr)
	if err != nil {
		return PeriodCategories{}, invalid(err)
	}
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return PeriodCategories{}, err
	}
	if c.IsSystem() {
		return PeriodCategories{}, fmt.Errorf("%w: %s is always used", ErrSystemCategory, c.Name)
	}

	current, err := s.resolve(ctx, p)
	if err != nil {
		return PeriodCategories{}, err
	}
	ids := slices.DeleteFunc(idsOf(current.Categories), func(other string) bool { return other == id })
	return s.replace(ctx, p, ids)
}

// resolve applies the inheritance rules: own → latest earlier configured period → catalog.
func (s *Service) resolve(ctx context.Context, p period.Period) (PeriodCategories, error) {
	own, err := s.periods.ListByPeriod(ctx, p)
	if err != nil {
		return PeriodCategories{}, err
	}
	if len(own) > 0 {
		return PeriodCategories{Period: p, Source: SourceOwn, Categories: own}, nil
	}

	prev, ok, err := s.periods.LatestConfiguredBefore(ctx, p)
	if err != nil {
		return PeriodCategories{}, err
	}
	if ok {
		inherited, err := s.periods.ListByPeriod(ctx, prev)
		if err != nil {
			return PeriodCategories{}, err
		}
		// Categories archived since then don't carry over to new periods.
		inherited = slices.DeleteFunc(inherited, func(c Category) bool { return c.Archived })
		return PeriodCategories{Period: p, Source: SourceInherited, InheritedFrom: prev, Categories: inherited}, nil
	}

	catalog, err := s.repo.List(ctx, false)
	if err != nil {
		return PeriodCategories{}, err
	}
	return PeriodCategories{Period: p, Source: SourceCatalog, Categories: catalog}, nil
}

// replace stores the period's own list, always including the system categories.
func (s *Service) replace(ctx context.Context, p period.Period, ids []string) (PeriodCategories, error) {
	catalog, err := s.repo.List(ctx, false)
	if err != nil {
		return PeriodCategories{}, err
	}
	for _, c := range catalog {
		if c.IsSystem() && !slices.Contains(ids, c.ID) {
			ids = append(ids, c.ID)
		}
	}
	if err := s.periods.ReplacePeriod(ctx, p, ids); err != nil {
		return PeriodCategories{}, fmt.Errorf("save period categories: %w", err)
	}
	return s.resolve(ctx, p)
}

func idsOf(cs []Category) []string {
	ids := make([]string, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, c.ID)
	}
	return ids
}
