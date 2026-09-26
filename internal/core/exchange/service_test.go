package exchange

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

// Fakes: minimal in-test implementations of the ports.

type fakeRates struct{ rates map[string]PeriodRate }

func rateKey(p period.Period, from, to money.Currency) string {
	return fmt.Sprintf("%s/%s/%s", p, from, to)
}

func (f *fakeRates) SaveRate(_ context.Context, r PeriodRate) error {
	f.rates[rateKey(r.Period, r.From, r.To)] = r
	return nil
}

func (f *fakeRates) FindRate(_ context.Context, p period.Period, from, to money.Currency) (PeriodRate, error) {
	r, ok := f.rates[rateKey(p, from, to)]
	if !ok {
		return PeriodRate{}, ErrRateNotFound
	}
	return r, nil
}

type fakeExchanges struct{ saved []Exchange }

func (f *fakeExchanges) Save(_ context.Context, e Exchange) error {
	f.saved = append(f.saved, e)
	return nil
}

type fakeIncomes struct{ saved []income.Income }

func (f *fakeIncomes) Save(_ context.Context, in income.Income) error {
	f.saved = append(f.saved, in)
	return nil
}

type fixture struct {
	svc       *Service
	exchanges *fakeExchanges
	incomes   *fakeIncomes
}

func newFixture() fixture {
	f := fixture{exchanges: &fakeExchanges{}, incomes: &fakeIncomes{}}
	f.svc = NewService(&fakeRates{rates: map[string]PeriodRate{}}, f.exchanges, f.incomes)
	f.svc.now = func() time.Time { return time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC) }
	ids := 0
	f.svc.newID = func() string { ids++; return fmt.Sprintf("id-%d", ids) }
	return f
}

var usdPen = PairInput{Period: "2026-07", From: "USD", To: "PEN"}

func (f fixture) setRate(t *testing.T, rate string) {
	t.Helper()
	if _, err := f.svc.SetRate(context.Background(), SetRateInput{PairInput: usdPen, Rate: rate}); err != nil {
		t.Fatalf("SetRate: %v", err)
	}
}

func TestSetRateAndGet(t *testing.T) {
	f := newFixture()
	f.setRate(t, "3.25")
	f.setRate(t, "3.30") // updating replaces the previous rate

	got, err := f.svc.Rate(context.Background(), usdPen)
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if got.Rate.String() != "3.3000" {
		t.Errorf("rate = %s, want 3.3000", got.Rate)
	}
}

func TestRateNotFound(t *testing.T) {
	_, err := newFixture().svc.Rate(context.Background(), usdPen)
	if !errors.Is(err, ErrRateNotFound) {
		t.Fatalf("err = %v, want %v", err, ErrRateNotFound)
	}
}

func TestSetRateValidation(t *testing.T) {
	tests := []struct {
		name string
		in   SetRateInput
	}{
		{"bad period", SetRateInput{PairInput{"2026-7", "USD", "PEN"}, "3.25"}},
		{"unsupported currency", SetRateInput{PairInput{"2026-07", "USD", "EUR"}, "3.25"}},
		{"same currency", SetRateInput{PairInput{"2026-07", "USD", "usd"}, "3.25"}},
		{"bad rate", SetRateInput{PairInput{"2026-07", "USD", "PEN"}, "3.25671"}},
		{"zero rate", SetRateInput{PairInput{"2026-07", "USD", "PEN"}, "0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newFixture().svc.SetRate(context.Background(), tt.in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	f := newFixture()
	f.setRate(t, "3.25")

	ex, err := f.svc.Register(context.Background(), RegisterInput{
		PairInput:   usdPen,
		Amount:      "1096.00",
		Description: "Casa de cambio",
		ExchangedOn: "2026-08-01",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if ex.From.String() != "1096.00" || ex.From.Currency() != money.USD {
		t.Errorf("From = %s %s, want 1096.00 USD", ex.From, ex.From.Currency())
	}
	if ex.To.String() != "3562.00" || ex.To.Currency() != money.PEN {
		t.Errorf("To = %s %s, want 3562.00 PEN", ex.To, ex.To.Currency())
	}
	if ex.Rate.String() != "3.2500" {
		t.Errorf("Rate = %s, want 3.2500", ex.Rate)
	}
	if len(f.exchanges.saved) != 1 {
		t.Fatalf("saved %d exchanges, want 1", len(f.exchanges.saved))
	}

	// The converted amount must be registered as an income in the target currency.
	if len(f.incomes.saved) != 1 {
		t.Fatalf("saved %d incomes, want 1", len(f.incomes.saved))
	}
	inc := f.incomes.saved[0]
	if inc.ID != ex.IncomeID {
		t.Errorf("income ID = %q, exchange.IncomeID = %q", inc.ID, ex.IncomeID)
	}
	if inc.Kind != income.KindExchange {
		t.Errorf("income kind = %q, want %q", inc.Kind, income.KindExchange)
	}
	if inc.Amount.String() != "3562.00" || inc.Amount.Currency() != money.PEN {
		t.Errorf("income amount = %s %s, want 3562.00 PEN", inc.Amount, inc.Amount.Currency())
	}
	if inc.Period.String() != "2026-07" {
		t.Errorf("income period = %s, want 2026-07", inc.Period)
	}
}

func TestRegisterTruncates(t *testing.T) {
	f := newFixture()
	f.setRate(t, "3.2567")

	ex, err := f.svc.Register(context.Background(), RegisterInput{
		PairInput: usdPen, Amount: "1096.37", ExchangedOn: "2026-08-01",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if ex.To.String() != "3570.54" { // 3570.548179 truncated
		t.Errorf("To = %s, want 3570.54", ex.To)
	}
}

func TestRegisterKeepsRateAfterUpdate(t *testing.T) {
	f := newFixture()
	f.setRate(t, "3.25")
	ex, err := f.svc.Register(context.Background(), RegisterInput{
		PairInput: usdPen, Amount: "100", ExchangedOn: "2026-08-01",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	f.setRate(t, "3.40")

	if ex.Rate.String() != "3.2500" || f.exchanges.saved[0].Rate.String() != "3.2500" {
		t.Errorf("exchange rate changed after updating the period rate")
	}
}

func TestRegisterWithoutRate(t *testing.T) {
	f := newFixture()
	_, err := f.svc.Register(context.Background(), RegisterInput{
		PairInput: usdPen, Amount: "100", ExchangedOn: "2026-08-01",
	})
	if !errors.Is(err, ErrRateNotFound) {
		t.Fatalf("err = %v, want %v", err, ErrRateNotFound)
	}
	if len(f.exchanges.saved)+len(f.incomes.saved) != 0 {
		t.Error("nothing must be saved when the rate is missing")
	}
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name string
		in   RegisterInput
	}{
		{"zero amount", RegisterInput{PairInput: usdPen, Amount: "0", ExchangedOn: "2026-08-01"}},
		{"bad amount", RegisterInput{PairInput: usdPen, Amount: "1.005", ExchangedOn: "2026-08-01"}},
		{"bad date", RegisterInput{PairInput: usdPen, Amount: "100", ExchangedOn: "01/08/2026"}},
		{"same currency", RegisterInput{PairInput: PairInput{"2026-07", "PEN", "PEN"}, Amount: "100", ExchangedOn: "2026-08-01"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.setRate(t, "3.25")

			_, err := f.svc.Register(context.Background(), tt.in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
			}
			if len(f.exchanges.saved)+len(f.incomes.saved) != 0 {
				t.Error("nothing must be saved on invalid input")
			}
		})
	}
}

func TestRegisterConvertsToZero(t *testing.T) {
	f := newFixture()
	f.setRate(t, "0.0001")

	// 0.01 USD × 0.0001 = 0.000001 PEN, truncated to 0.00: nothing would be received.
	_, err := f.svc.Register(context.Background(), RegisterInput{
		PairInput: usdPen, Amount: "0.01", ExchangedOn: "2026-08-01",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
	}
}
