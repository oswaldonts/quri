package memory

import (
	"context"
	"sync"
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
)

func TestIncomeRepositorySaveConcurrent(t *testing.T) {
	repo := NewIncomeRepository()

	const n = 100
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			if err := repo.Save(context.Background(), income.Income{}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()

	if got := len(repo.incomes); got != n {
		t.Errorf("stored %d incomes, want %d", got, n)
	}
}
