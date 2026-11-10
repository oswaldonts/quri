package expense

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/account"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/status"
)

type fakeRepo struct{ saved []Expense }

func (f *fakeRepo) Save(_ context.Context, e Expense) error {
	f.saved = append(f.saved, e)
	return nil
}

func (f *fakeRepo) SaveAll(_ context.Context, es []Expense) error {
	f.saved = append(f.saved, es...)
	return nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (Expense, error) {
	for _, e := range f.saved {
		if e.ID == id {
			return e, nil
		}
	}
	return Expense{}, ErrNotFound
}

func (f *fakeRepo) Update(_ context.Context, e Expense) error {
	for i := range f.saved {
		if f.saved[i].ID == e.ID {
			f.saved[i] = e
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	for i := range f.saved {
		if f.saved[i].ID == id {
			f.saved = append(f.saved[:i], f.saved[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) ListByPeriod(_ context.Context, p period.Period) ([]Expense, error) {
	var out []Expense
	for _, e := range f.saved {
		if e.Period == p {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeRepo) StartSeries(_ context.Context, first Expense, following []Expense) error {
	if err := f.Update(context.Background(), first); err != nil {
		return err
	}
	f.saved = append(f.saved, following...)
	return nil
}

func (f *fakeRepo) DeleteSeriesFrom(_ context.Context, seriesID string, from period.Period) error {
	f.saved = slices.DeleteFunc(f.saved, func(e Expense) bool {
		return e.SeriesID == seriesID && e.Period.Compare(from) >= 0
	})
	return nil
}

// fakeCategories knows which categories each period uses.
type fakeCategories struct{ inPeriod map[string]category.Category }

func (f fakeCategories) EnsureInPeriod(_ context.Context, _ period.Period, id string) (category.Category, error) {
	if id == "unused" {
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotInPeriod, id)
	}
	c, ok := f.inPeriod[id]
	if !ok {
		return category.Category{}, fmt.Errorf("%w: %q", category.ErrNotFound, id)
	}
	return c, nil
}

// fakeAccounts knows a bank account and the cash, both in PEN.
type fakeAccounts struct{}

func (fakeAccounts) Get(_ context.Context, id string) (account.Account, error) {
	pen := func(kind account.Kind) account.Account {
		return account.Account{ID: id, Name: id, Kind: kind, Amount: money.Zero(money.PEN)}
	}
	switch id {
	case "bcp":
		return pen(account.Bank), nil
	case "cash":
		return pen(account.Cash), nil
	case "usd":
		a := pen(account.Bank)
		a.Amount = money.Zero(money.USD)
		return a, nil
	}
	return account.Account{}, fmt.Errorf("%w: %q", account.ErrNotFound, id)
}

func newTestService(repo *fakeRepo) *Service {
	s := NewService(repo, fakeCategories{inPeriod: map[string]category.Category{
		"market":   {ID: "market", Name: "Mercado", Group: category.GroupNeeds},
		"exchange": {ID: "exchange", Name: "Cambio", Group: category.GroupSystem},
	}}, fakeAccounts{}, anyCurrency{})
	s.now = func() time.Time { return time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC) }
	ids := 0
	s.newID = func() string { ids++; return fmt.Sprintf("exp-%d", ids) }
	return s
}

func validInput() RegisterInput {
	return RegisterInput{
		Period:      "2026-07",
		CategoryID:  "market",
		Currency:    "PEN",
		Amount:      "45.50",
		Description: " Mercado semanal ",
		SpentOn:     "2026-08-03",
	}
}

func TestRegister(t *testing.T) {
	repo := &fakeRepo{}
	got, err := newTestService(repo).Register(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.ID != "exp-1" || got.CategoryID != "market" || got.Amount.String() != "45.50" ||
		got.Amount.Currency() != "PEN" || got.Description != "Mercado semanal" || got.ExchangeID != "" {
		t.Errorf("Register = %+v", got)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("saved %d expenses, want 1", len(repo.saved))
	}

	list, err := newTestService(repo).List(context.Background(), "2026-07", Filter{})
	if err != nil || len(list) != 1 {
		t.Errorf("List = %d expenses, %v; want 1", len(list), err)
	}
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*RegisterInput)
	}{
		{"bad period", func(in *RegisterInput) { in.Period = "2026-7" }},
		{"unregistered currency", func(in *RegisterInput) { in.Currency = "EUR" }},
		{"bad amount", func(in *RegisterInput) { in.Amount = "1.005" }},
		{"zero amount", func(in *RegisterInput) { in.Amount = "0" }},
		{"long description", func(in *RegisterInput) { in.Description = strings.Repeat("a", 201) }},
		{"bad date", func(in *RegisterInput) { in.SpentOn = "03/08/2026" }},
		{"unknown category", func(in *RegisterInput) { in.CategoryID = "nope" }},
		{"category not used in the period", func(in *RegisterInput) { in.CategoryID = "unused" }},
		{"system category", func(in *RegisterInput) { in.CategoryID = "exchange" }},
		{"empty tag", func(in *RegisterInput) { in.Tags = []string{"dulces", "  "} }},
		{"long tag", func(in *RegisterInput) { in.Tags = []string{strings.Repeat("a", 31)} }},
		{"too many tags", func(in *RegisterInput) {
			in.Tags = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			in := validInput()
			tt.modify(&in)

			_, err := newTestService(repo).Register(context.Background(), in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
			}
			if len(repo.saved) != 0 {
				t.Error("nothing must be saved on invalid input")
			}
		})
	}
}

func TestListBadPeriod(t *testing.T) {
	if _, err := newTestService(&fakeRepo{}).List(context.Background(), "julio", Filter{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
	}
}

func TestNormalizeTags(t *testing.T) {
	got, err := NormalizeTags([]string{"#Dulces", " dulces ", "Frutos   Secos", "snacks"})
	if err != nil {
		t.Fatalf("NormalizeTags: %v", err)
	}
	want := []string{"dulces", "frutos secos", "snacks"}
	if !slices.Equal(got, want) {
		t.Errorf("NormalizeTags = %q, want %q", got, want)
	}
	if got, _ := NormalizeTags(nil); len(got) != 0 {
		t.Errorf("NormalizeTags(nil) = %q, want empty", got)
	}
}

func TestRegisterWithTags(t *testing.T) {
	in := validInput()
	in.Tags = []string{"Lácteos", "#desayuno"}
	got, err := newTestService(&fakeRepo{}).Register(context.Background(), in)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if want := []string{"desayuno", "lácteos"}; !slices.Equal(got.Tags, want) {
		t.Errorf("Tags = %q, want %q", got.Tags, want)
	}
}

func validBatch() BatchInput {
	return BatchInput{
		Period:     "2026-07",
		CategoryID: "market",
		Currency:   "PEN",
		SpentOn:    "2026-08-03",
		Tags:       []string{"mercado"},
		Items: []BatchItem{
			{Description: "Chocolate Milky", Amount: "27", Tags: []string{"dulces"}},
			{Description: "Queso", Amount: "37"},
			{Description: "Pan", Amount: "9", SpentOn: "2026-08-04"},
		},
	}
}

func TestRegisterBatch(t *testing.T) {
	repo := &fakeRepo{}
	got, err := newTestService(repo).RegisterBatch(context.Background(), validBatch())
	if err != nil {
		t.Fatalf("RegisterBatch: %v", err)
	}
	if len(got) != 3 || len(repo.saved) != 3 {
		t.Fatalf("registered %d, saved %d; want 3 and 3", len(got), len(repo.saved))
	}
	// Common tags are added to the item's own tags.
	if !slices.Equal(got[0].Tags, []string{"dulces", "mercado"}) || !slices.Equal(got[1].Tags, []string{"mercado"}) {
		t.Errorf("tags = %q and %q", got[0].Tags, got[1].Tags)
	}
	// Items inherit the common fields unless they set their own.
	if got[1].SpentOn.Format(time.DateOnly) != "2026-08-03" || got[2].SpentOn.Format(time.DateOnly) != "2026-08-04" {
		t.Errorf("spent_on = %s and %s", got[1].SpentOn, got[2].SpentOn)
	}
	if got[0].CategoryID != "market" || got[0].Amount.Currency() != "PEN" || got[0].ID == got[1].ID {
		t.Errorf("first item = %+v", got[0])
	}
}

func TestRegisterBatchIsAllOrNothing(t *testing.T) {
	tests := []struct {
		name     string
		modify   func(*BatchInput)
		wantItem string
	}{
		{"bad amount in item 2", func(b *BatchInput) { b.Items[1].Amount = "abc" }, "item 2"},
		{"category not used, item 3", func(b *BatchInput) { b.Items[2].CategoryID = "unused" }, "item 3"},
		{"no items", func(b *BatchInput) { b.Items = nil }, ""},
		{"too many items", func(b *BatchInput) { b.Items = make([]BatchItem, 101) }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			in := validBatch()
			tt.modify(&in)

			_, err := newTestService(repo).RegisterBatch(context.Background(), in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want %v", err, ErrInvalidInput)
			}
			if tt.wantItem != "" && !strings.Contains(err.Error(), tt.wantItem) {
				t.Errorf("err = %v, want it to name %s", err, tt.wantItem)
			}
			if len(repo.saved) != 0 {
				t.Errorf("saved %d expenses, want none", len(repo.saved))
			}
		})
	}
}

func TestListFilters(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo)
	if _, err := svc.RegisterBatch(ctx, validBatch()); err != nil {
		t.Fatal(err)
	}
	other := validInput()
	other.CategoryID = "leisure"
	svc.categories.(fakeCategories).inPeriod["leisure"] = category.Category{ID: "leisure", Group: category.GroupLifestyle}
	if _, err := svc.Register(ctx, other); err != nil {
		t.Fatal(err)
	}

	count := func(f Filter) int {
		t.Helper()
		es, err := svc.List(ctx, "2026-07", f)
		if err != nil {
			t.Fatalf("List(%+v): %v", f, err)
		}
		return len(es)
	}
	if n := count(Filter{}); n != 4 {
		t.Errorf("no filter = %d, want 4", n)
	}
	if n := count(Filter{CategoryID: "market"}); n != 3 {
		t.Errorf("category market = %d, want 3", n)
	}
	if n := count(Filter{Tag: "#Dulces"}); n != 1 {
		t.Errorf("tag dulces = %d, want 1", n)
	}
	if n := count(Filter{CategoryID: "leisure", Tag: "mercado"}); n != 0 {
		t.Errorf("leisure + mercado = %d, want 0", n)
	}
}

func ptr[T any](v T) *T { return &v }

func TestUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo)
	in := validInput()
	in.Tags = []string{"dulces"}
	created, err := svc.Register(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.Update(ctx, "2026-07", created.ID, UpdateInput{Amount: ptr("54.50"), Tags: ptr([]string{"Lácteos"})})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Amount.String() != "54.50" || !slices.Equal(got.Tags, []string{"lácteos"}) ||
		got.Description != "Mercado semanal" || got.ID != created.ID {
		t.Errorf("Update = %+v", got)
	}
	// Tags: nil keeps them, an empty list clears them.
	if got, _ := svc.Update(ctx, "2026-07", created.ID, UpdateInput{Description: ptr("x")}); !slices.Equal(got.Tags, []string{"lácteos"}) {
		t.Errorf("tags after update without tags = %q", got.Tags)
	}
	if got, _ := svc.Update(ctx, "2026-07", created.ID, UpdateInput{Tags: ptr([]string{})}); len(got.Tags) != 0 {
		t.Errorf("tags after clearing = %q", got.Tags)
	}

	if _, err := svc.Update(ctx, "2026-07", created.ID, UpdateInput{CategoryID: ptr("exchange")}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("move to system category: err = %v, want %v", err, ErrInvalidInput)
	}
	if err := svc.Delete(ctx, "2026-07", created.ID); err != nil || len(repo.saved) != 0 {
		t.Errorf("Delete = %v, remaining %d", err, len(repo.saved))
	}
}

func TestUpdateAndDeleteErrors(t *testing.T) {
	ctx := context.Background()
	july, _ := period.Parse("2026-07")
	repo := &fakeRepo{saved: []Expense{
		{ID: "normal", Period: july, CategoryID: "market"},
		{ID: "from-exchange", Period: july, CategoryID: "exchange", ExchangeID: "ex-1"},
	}}
	svc := newTestService(repo)

	if _, err := svc.Update(ctx, "2026-07", "nope", UpdateInput{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: err = %v, want %v", err, ErrNotFound)
	}
	if err := svc.Delete(ctx, "2026-08", "normal"); !errors.Is(err, ErrNotFound) {
		t.Errorf("other period: err = %v, want %v", err, ErrNotFound)
	}
	if _, err := svc.Update(ctx, "2026-07", "from-exchange", UpdateInput{}); !errors.Is(err, ErrManagedByExchange) {
		t.Errorf("exchange expense: err = %v, want %v", err, ErrManagedByExchange)
	}
	if err := svc.Delete(ctx, "2026-07", "from-exchange"); !errors.Is(err, ErrManagedByExchange) {
		t.Errorf("delete exchange expense: err = %v, want %v", err, ErrManagedByExchange)
	}
	if err := svc.Delete(ctx, "julio", "normal"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad period: err = %v, want %v", err, ErrInvalidInput)
	}
}

func TestStatusAndFilter(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(&fakeRepo{})

	in := validInput()
	in.Status = "planned"
	planned, err := svc.Register(ctx, in)
	if err != nil || planned.Status != status.Planned {
		t.Fatalf("planned = %q, %v", planned.Status, err)
	}
	if _, err := svc.Register(ctx, validInput()); err != nil { // confirmed by default
		t.Fatal(err)
	}
	batch := validBatch()
	batch.Status = "planned"
	batch.Items[0].Status = "confirmed" // an item can override the common status
	if _, err := svc.RegisterBatch(ctx, batch); err != nil {
		t.Fatal(err)
	}

	count := func(st string) int {
		es, err := svc.List(ctx, "2026-07", Filter{Status: st})
		if err != nil {
			t.Fatal(err)
		}
		return len(es)
	}
	if count("planned") != 3 || count("confirmed") != 2 || count("") != 5 {
		t.Errorf("planned %d, confirmed %d, all %d; want 3, 2, 5", count("planned"), count("confirmed"), count(""))
	}
	if _, err := svc.List(ctx, "2026-07", Filter{Status: "pending"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("bad status filter: err = %v, want %v", err, ErrInvalidInput)
	}

	got, err := svc.Update(ctx, "2026-07", planned.ID, UpdateInput{Status: ptr("confirmed"), Amount: ptr("46.00")})
	if err != nil || got.Status != status.Confirmed || got.Amount.String() != "46.00" {
		t.Errorf("confirm = %+v, %v", got, err)
	}
}

func TestRegisterChargesAnAccount(t *testing.T) {
	tests := []struct {
		name    string
		account string
		wantErr error
	}{
		{"no account", "", nil},
		{"bank", "bcp", nil},
		{"cash", "cash", nil},
		{"unknown", "nope", ErrInvalidInput},
		{"other currency", "usd", ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			in.AccountID = tt.account

			e, err := newTestService(&fakeRepo{}).Register(context.Background(), in)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && e.AccountID != tt.account {
				t.Errorf("expense = %+v, want account %q", e, tt.account)
			}
		})
	}
}

// anyCurrency accepts every currency except EUR, which tests use as the unregistered one.
type anyCurrency struct{}

func (anyCurrency) Require(_ context.Context, c money.Currency) error {
	if c == "EUR" {
		return ErrInvalidInput // stands for a currency that was never registered
	}
	return nil
}

func TestRegisterRecurring(t *testing.T) {
	repo := &fakeRepo{}
	in := validInput()
	in.Period, in.SpentOn, in.Occurrences, in.Tags = "2026-01", "2026-01-31", 3, []string{"seguro"}
	first, err := newTestService(repo).Register(context.Background(), in)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(repo.saved) != 3 {
		t.Fatalf("saved %d expenses, want 3", len(repo.saved))
	}
	wantPeriod := []string{"2026-01", "2026-02", "2026-03"}
	wantDate := []string{"2026-01-31", "2026-02-28", "2026-03-31"}
	for i, e := range repo.saved {
		if e.Period.String() != wantPeriod[i] || e.SpentOn.Format(time.DateOnly) != wantDate[i] ||
			e.SeriesID != first.SeriesID || e.SeriesNo != i+1 || e.SeriesTotal != 3 ||
			e.Amount.String() != "45.50" || !slices.Equal(e.Tags, []string{"seguro"}) {
			t.Errorf("occurrence %d = %+v", i+1, e)
		}
		if i > 0 && e.Status != status.Planned {
			t.Errorf("occurrence %d status = %s, want planned", i+1, e.Status)
		}
	}
	if first.SeriesID == "" || first.Status != status.Confirmed {
		t.Errorf("first = %+v", first)
	}
}

func TestRegisterOccurrencesOutOfRange(t *testing.T) {
	for _, n := range []int{-1, MaxOccurrences + 1} {
		in := validInput()
		in.Occurrences = n
		repo := &fakeRepo{}
		if _, err := newTestService(repo).Register(context.Background(), in); !errors.Is(err, ErrInvalidInput) || len(repo.saved) != 0 {
			t.Errorf("Occurrences %d: err = %v, saved %d", n, err, len(repo.saved))
		}
	}
}

func TestRepeatAndDeleteFollowing(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo)
	e, err := svc.Register(ctx, validInput())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := svc.Repeat(ctx, "2026-07", e.ID, 1, 1); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("Repeat 1 time: err = %v, want ErrInvalidInput", err)
	}
	first, err := svc.Repeat(ctx, "2026-07", e.ID, 1, 4)
	if err != nil {
		t.Fatalf("Repeat: %v", err)
	}
	if len(repo.saved) != 4 || first.SeriesNo != 1 || first.SeriesTotal != 4 || repo.saved[0].SeriesID != first.SeriesID {
		t.Fatalf("after Repeat: %d saved, first = %+v", len(repo.saved), first)
	}
	// Repeating again replaces the later installments.
	again, err := svc.Repeat(ctx, "2026-07", e.ID, 2, 3)
	if err != nil || again.SeriesNo != 2 || again.SeriesID == first.SeriesID || len(repo.saved) != 2 || repo.saved[1].SeriesNo != 3 {
		t.Fatalf("Repeat again = %+v, %v, %d saved", again, err, len(repo.saved))
	}
	if _, err := svc.Repeat(ctx, "2026-07", e.ID, 1, 4); err != nil {
		t.Fatalf("Repeat 1 of 4: %v", err)
	}
	if _, err := svc.Repeat(ctx, "2026-07", e.ID, 1, 1); err != nil || len(repo.saved) != 1 || repo.saved[0].SeriesID != "" {
		t.Fatalf("Repeat 1 of 1: err = %v, saved %+v, want a plain expense", err, repo.saved)
	}
	if first, err = svc.Repeat(ctx, "2026-07", e.ID, 1, 4); err != nil {
		t.Fatalf("Repeat 1 of 4 again: %v", err)
	}

	// Editing one installment keeps it in the series.
	updated, err := svc.Update(ctx, "2026-08", repo.saved[1].ID, UpdateInput{Amount: new("50")})
	if err != nil || updated.SeriesID != first.SeriesID || updated.SeriesNo != 2 {
		t.Errorf("Update = %+v, %v", updated, err)
	}

	if err := svc.DeleteFollowing(ctx, "2026-08", repo.saved[1].ID); err != nil {
		t.Fatalf("DeleteFollowing: %v", err)
	}
	if len(repo.saved) != 1 || repo.saved[0].ID != e.ID {
		t.Errorf("left %+v, want only the first occurrence", repo.saved)
	}
}

func TestRepeatFromMiddleInstallment(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := newTestService(repo)
	e, _ := svc.Register(ctx, validInput())
	if _, err := svc.Repeat(ctx, "2026-07", e.ID, 13, 12); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("installment 13 of 12: err = %v, want ErrInvalidInput", err)
	}
	first, err := svc.Repeat(ctx, "2026-07", e.ID, 9, 12)
	if err != nil {
		t.Fatalf("Repeat: %v", err)
	}
	if first.SeriesNo != 9 || len(repo.saved) != 4 {
		t.Fatalf("first = %+v, %d saved, want installment 9 and 3 more", first, len(repo.saved))
	}
	if last := repo.saved[3]; last.SeriesNo != 12 || last.SeriesTotal != 12 || last.Period.String() != "2026-10" {
		t.Errorf("last = %+v", last)
	}
}
