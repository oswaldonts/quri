package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/oswaldoaquinas/go-income-mgr/internal/adapter/sqlite/sqlitedb"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/summary"
)

// Compile-time check that PeriodBounds implements the core port.
var _ summary.PeriodBounds = (*PeriodBounds)(nil)

// PeriodBounds answers where the history starts.
type PeriodBounds struct {
	q *sqlitedb.Queries
}

// NewPeriodBounds returns a PeriodBounds backed by db.
func NewPeriodBounds(db *sql.DB) *PeriodBounds {
	return &PeriodBounds{q: sqlitedb.New(db)}
}

// EarliestPeriod returns the first period with incomes, expenses or balances.
func (b *PeriodBounds) EarliestPeriod(ctx context.Context) (period.Period, bool, error) {
	s, err := b.q.EarliestPeriod(ctx)
	if err != nil {
		return period.Period{}, false, err
	}
	if s == "" {
		return period.Period{}, false, nil
	}
	p, err := period.Parse(s)
	if err != nil {
		return period.Period{}, false, fmt.Errorf("earliest period: %w", err)
	}
	return p, true, nil
}
