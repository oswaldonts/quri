// Package memory provides in-memory repositories. Data is lost when the process stops.
package memory

import (
	"context"
	"sync"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

// Compile-time check that IncomeRepository implements the core port.
var _ income.Repository = (*IncomeRepository)(nil)

// IncomeRepository stores incomes in a slice. It is safe for concurrent use:
// the HTTP server runs each request in its own goroutine.
type IncomeRepository struct {
	mu      sync.Mutex
	incomes []income.Income
}

// NewIncomeRepository returns an empty repository.
func NewIncomeRepository() *IncomeRepository {
	return &IncomeRepository{}
}

// Save appends the income.
func (r *IncomeRepository) Save(_ context.Context, in income.Income) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.incomes = append(r.incomes, in)
	return nil
}
