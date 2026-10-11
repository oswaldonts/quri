package exchange

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/income"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
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

func (f *fakeRates) DeleteRate(_ context.Context, p period.Period, from, to money.Currency) error {
	if _, ok := f.rates[rateKey(p, from, to)]; !ok {
		return ErrRateNotFound
	}
	delete(f.rates, rateKey(p, from, to))
	return nil
}

func (f *fakeRates) FindRate(_ context.Context, p period.Period, from, to money.Currency) (PeriodRate, error) {
	r, ok := f.rates[rateKey(p, from, to)]
	if !ok {
		return PeriodRate{}, ErrRateNotFound
	}
	return r, nil
}

// fakeExchanges records exchanges and the incomes and expenses saved with them.
type fakeExchanges struct {
	saved    []Exchange
	incomes  []income.Income
	expenses []expense.Expense
}

func (f *fakeExchanges) Save(_ context.Context, e Exchange, in income.Income, out expense.Expense) error {
	f.saved = append(f.saved, e)
	f.incomes = append(f.incomes, in)
	f.expenses = append(f.expenses, out)
	return nil
}

func (f *fakeExchanges) Get(_ context.Context, id string) (Exchange, error) {
	for _, e := range f.saved {
		if e.ID == id {
			return e, nil
		}
	}
	return Exchange{}, ErrNotFound
}

func (f *fakeExchanges) ListByPeriod(_ context.Context, p period.Period) ([]Exchange, error) {
	var out []Exchange
	for _, e := range f.saved {
		if e.Period == p {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeExchanges) Update(_ context.Context, e Exchange, in income.Income, out expense.Expense) error {
	for i := range f.saved {
		if f.saved[i].ID == e.ID {
			f.saved[i], f.incomes[i] = e, in
			out.ID = f.expenses[i].ID
			f.expenses[i] = out
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeExchanges) Delete(_ context.Context, e Exchange) error {
	for i := range f.saved {
		if f.saved[i].ID == e.ID {
			f.saved = append(f.saved[:i], f.saved[i+1:]...)
			f.incomes = append(f.incomes[:i], f.incomes[i+1:]...)
			f.expenses = append(f.expenses[:i], f.expenses[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// allCategories accepts every category in every period.
type allCategories struct{}

func (allCategories) EnsureInPeriod(_ context.Context, _ period.Period, id string) (category.Category, error) {
	return category.Category{ID: id, Group: category.GroupSystem}, nil
}

type fixture struct {
	svc       *Service
	exchanges *fakeExchanges
}

func newFixture() fixture {
	f := fixture{exchanges: &fakeExchanges{}}
	f.svc = NewService(&fakeRates{rates: map[string]PeriodRate{}}, f.exchanges, allCategories{}, anyCurrency{})
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
	if got.Rate.String() != "3.3" {
		t.Errorf("rate = %s, want 3.3", got.Rate)
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
		{"unregistered currency", SetRateInput{PairInput{"2026-07", "USD", "EUR"}, "3.25"}},
		{"same currency", SetRateInput{PairInput{"2026-07", "USD", "usd"}, "3.25"}},
		{"bad rate", SetRateInput{PairInput{"2026-07", "USD", "PEN"}, "0.123456789"}},
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
	if ex.Rate.String() != "3.25" {
		t.Errorf("Rate = %s, want 3.25", ex.Rate)
	}
	if len(f.exchanges.saved) != 1 {
		t.Fatalf("saved %d exchanges, want 1", len(f.exchanges.saved))
	}

	// The converted amount must be registered as an income in the target currency.
	if len(f.exchanges.incomes) != 1 {
		t.Fatalf("saved %d incomes, want 1", len(f.exchanges.incomes))
	}
	inc := f.exchanges.incomes[0]
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

	// The source amount must be registered as an expense in the exchange category.
	if len(f.exchanges.expenses) != 1 {
		t.Fatalf("saved %d expenses, want 1", len(f.exchanges.expenses))
	}
	out := f.exchanges.expenses[0]
	if out.CategoryID != category.ExchangeCategoryID || out.ExchangeID != ex.ID ||
		out.Amount.String() != "1096.00" || out.Amount.Currency() != money.USD {
		t.Errorf("expense = %+v, want 1096.00 USD in %q linked to the exchange", out, category.ExchangeCategoryID)
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

	if ex.Rate.String() != "3.25" || f.exchanges.saved[0].Rate.String() != "3.25" {
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
	if len(f.exchanges.saved)+len(f.exchanges.incomes) != 0 {
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
			if len(f.exchanges.saved)+len(f.exchanges.incomes) != 0 {
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

func TestListAndDelete(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.setRate(t, "3.25")
	ex, err := f.svc.Register(ctx, RegisterInput{PairInput: usdPen, Amount: "100", ExchangedOn: "2026-08-01"})
	if err != nil {
		t.Fatal(err)
	}

	list, err := f.svc.List(ctx, "2026-07")
	if err != nil || len(list) != 1 || list[0].ID != ex.ID {
		t.Fatalf("List = %+v, %v", list, err)
	}

	if err := f.svc.Delete(ctx, "2026-08", ex.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("other period: err = %v, want %v", err, ErrNotFound)
	}
	if err := f.svc.Delete(ctx, "julio", ex.ID); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad period: err = %v, want %v", err, ErrInvalidInput)
	}
	if _, err := f.svc.List(ctx, "julio"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("List bad period: err = %v, want %v", err, ErrInvalidInput)
	}
	if err := f.svc.Delete(ctx, "2026-07", ex.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(f.exchanges.saved)+len(f.exchanges.incomes)+len(f.exchanges.expenses) != 0 {
		t.Error("the exchange, its income and its expense must all be deleted")
	}
}

func TestDeleteRate(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.setRate(t, "3.25")

	if err := f.svc.DeleteRate(ctx, usdPen); err != nil {
		t.Fatalf("DeleteRate: %v", err)
	}
	if _, err := f.svc.Rate(ctx, usdPen); !errors.Is(err, ErrRateNotFound) {
		t.Errorf("after delete: err = %v, want %v", err, ErrRateNotFound)
	}
	if err := f.svc.DeleteRate(ctx, usdPen); !errors.Is(err, ErrRateNotFound) {
		t.Errorf("second delete: err = %v, want %v", err, ErrRateNotFound)
	}
	if err := f.svc.DeleteRate(ctx, PairInput{Period: "2026-7", From: "USD", To: "PEN"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad period: err = %v, want %v", err, ErrInvalidInput)
	}
}

func TestPlannedExchangeConfirmed(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.setRate(t, "3.25")

	ex, err := f.svc.Register(ctx, RegisterInput{PairInput: usdPen, Amount: "1100", ExchangedOn: "2026-08-01", Status: "planned"})
	if err != nil {
		t.Fatal(err)
	}
	if ex.Status != status.Planned || f.exchanges.incomes[0].Status != status.Planned || f.exchanges.expenses[0].Status != status.Planned {
		t.Fatalf("planned exchange must plan its income and expense too")
	}

	// It finally happened with 1200 USD, a week later: the rate stays 3.25.
	got, err := f.svc.Update(ctx, "2026-07", ex.ID, UpdateInput{Status: ptr("confirmed"), Amount: ptr("1200"), ExchangedOn: ptr("2026-08-08")})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Status != status.Confirmed || got.From.String() != "1200.00" || got.To.String() != "3900.00" || got.Rate.String() != "3.25" {
		t.Errorf("confirmed exchange = %+v", got)
	}
	inc, out := f.exchanges.incomes[0], f.exchanges.expenses[0]
	if inc.Status != status.Confirmed || inc.Amount.String() != "3900.00" || inc.ReceivedOn.Format("2006-01-02") != "2026-08-08" {
		t.Errorf("income = %+v, want confirmed 3900.00 on 2026-08-08", inc)
	}
	if out.Status != status.Confirmed || out.Amount.String() != "1200.00" {
		t.Errorf("expense = %+v, want confirmed 1200.00", out)
	}

	if _, err := f.svc.Update(ctx, "2026-08", ex.ID, UpdateInput{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("other period: err = %v, want %v", err, ErrNotFound)
	}
	if _, err := f.svc.Update(ctx, "2026-07", ex.ID, UpdateInput{Amount: ptr("0")}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("zero amount: err = %v, want %v", err, ErrInvalidInput)
	}
	if _, err := f.svc.Update(ctx, "2026-07", ex.ID, UpdateInput{Status: ptr("pending")}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad status: err = %v, want %v", err, ErrInvalidInput)
	}
}

func ptr[T any](v T) *T { return &v }

// anyCurrency accepts every currency except EUR, which tests use as the unregistered one.
type anyCurrency struct{}

func (anyCurrency) Require(_ context.Context, c money.Currency) error {
	if c == "EUR" {
		return ErrInvalidInput // stands for a currency that was never registered
	}
	return nil
}
