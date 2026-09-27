// Package period models budget periods: money earned in one month is spent in the next.
package period

import (
	"cmp"
	"errors"
	"fmt"
	"time"
)

var ErrInvalidPeriod = errors.New("invalid period")

// layout is Go's reference date (2006-01-02) written in the YYYY-MM format.
const layout = "2006-01"

// Period identifies a budget by the month the money was earned (e.g. 2026-07).
// That money is spent during the following month (see SpendMonth).
type Period struct {
	year  int
	month time.Month
}

// Parse reads a period in YYYY-MM format.
func Parse(s string) (Period, error) {
	t, err := time.Parse(layout, s)
	if err != nil {
		return Period{}, fmt.Errorf("%w: %q (want YYYY-MM)", ErrInvalidPeriod, s)
	}
	return Period{year: t.Year(), month: t.Month()}, nil
}

// Year returns the year the money was earned.
func (p Period) Year() int { return p.year }

// Month returns the month the money was earned.
func (p Period) Month() time.Month { return p.month }

// SpendMonth returns the month in which this period's money is spent.
func (p Period) SpendMonth() Period {
	// time.Date normalizes month 13 into January of the next year.
	t := time.Date(p.year, p.month+1, 1, 0, 0, 0, 0, time.UTC)
	return Period{year: t.Year(), month: t.Month()}
}

// Next returns the following period (same as SpendMonth, named for iteration).
func (p Period) Next() Period { return p.SpendMonth() }

// Compare returns -1, 0 or +1 when p is before, equal to or after q.
func (p Period) Compare(q Period) int {
	return cmp.Or(cmp.Compare(p.year, q.year), cmp.Compare(p.month, q.month))
}

// MonthsBetween returns how many periods there are from p to q, both included
// (0 if q is before p).
func (p Period) MonthsBetween(q Period) int {
	n := (q.year-p.year)*12 + int(q.month) - int(p.month) + 1
	return max(n, 0)
}

// String formats the period as YYYY-MM.
func (p Period) String() string {
	return fmt.Sprintf("%04d-%02d", p.year, int(p.month))
}
