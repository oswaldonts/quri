package memory

import (
	"context"
	"sync"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/exchange"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// Compile-time checks that the repositories implement the core ports.
var (
	_ exchange.RateRepository = (*RateRepository)(nil)
	_ exchange.Repository     = (*ExchangeRepository)(nil)
)

// rateKey identifies a rate. Structs with comparable fields can be used as map keys.
type rateKey struct {
	period period.Period
	from   money.Currency
	to     money.Currency
}

// RateRepository keeps one rate per period and currency pair.
type RateRepository struct {
	mu    sync.RWMutex
	rates map[rateKey]exchange.PeriodRate
}

// NewRateRepository returns an empty repository.
func NewRateRepository() *RateRepository {
	return &RateRepository{rates: make(map[rateKey]exchange.PeriodRate)}
}

// SaveRate creates or replaces the rate of its period and pair.
func (r *RateRepository) SaveRate(_ context.Context, pr exchange.PeriodRate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rates[rateKey{pr.Period, pr.From, pr.To}] = pr
	return nil
}

// FindRate returns exchange.ErrRateNotFound when the pair has no rate in the period.
func (r *RateRepository) FindRate(_ context.Context, p period.Period, from, to money.Currency) (exchange.PeriodRate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pr, ok := r.rates[rateKey{p, from, to}]
	if !ok {
		return exchange.PeriodRate{}, exchange.ErrRateNotFound
	}
	return pr, nil
}

// ExchangeRepository stores exchanges in a slice.
type ExchangeRepository struct {
	mu        sync.Mutex
	exchanges []exchange.Exchange
}

// NewExchangeRepository returns an empty repository.
func NewExchangeRepository() *ExchangeRepository {
	return &ExchangeRepository{}
}

// Save appends the exchange.
func (r *ExchangeRepository) Save(_ context.Context, e exchange.Exchange) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.exchanges = append(r.exchanges, e)
	return nil
}
