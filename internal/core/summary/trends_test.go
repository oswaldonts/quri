package summary

import (
	"testing"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/expense"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/money"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
)

func TestBuildTrends(t *testing.T) {
	jun, _ := period.Parse("2026-06")
	aug, _ := period.Parse("2026-08")

	// June: Mercado 100 (dulces). July: nothing. August: Mercado 50, Ocio 20.
	june := build(jun, entries{expenses: []expense.Expense{
		exp(t, "market", "100", money.PEN, "Compra", "dulces"),
	}}, catalog)
	august := build(aug, entries{expenses: []expense.Expense{
		exp(t, "market", "50", money.PEN, "Compra"),
		exp(t, "leisure", "20", money.PEN, "Cine"),
	}}, catalog)
	months := []CurrencySummary{
		currencyOf(june, money.PEN),
		currencyOf(Summary{}, money.PEN), // July had no activity
		currencyOf(august, money.PEN),
	}

	tr := buildTrends(jun, aug, money.PEN, months)

	if tr.Months != 3 || len(tr.Periods) != 3 || tr.Periods[1].Period.String() != "2026-07" {
		t.Fatalf("periods = %+v", tr.Periods)
	}
	if tr.Periods[1].Spent.String() != "0.00" {
		t.Errorf("July spent = %s, want 0.00 (empty month)", tr.Periods[1].Spent)
	}

	market := tr.Categories[0]
	if market.Category.ID != "market" || market.Total.String() != "150.00" || market.Average.String() != "50.00" {
		t.Errorf("Mercado = total %s average %s, want 150.00 and 50.00", market.Total, market.Average)
	}
	want := []string{"100.00", "0.00", "50.00"}
	for i, m := range market.Series {
		if m.String() != want[i] {
			t.Errorf("Mercado series[%d] = %s, want %s", i, m, want[i])
		}
	}

	leisure := tr.Categories[1]
	if leisure.Category.ID != "leisure" || leisure.Average.String() != "6.66" { // 20 / 3, truncated
		t.Errorf("Ocio = %s average %s, want leisure 6.66", leisure.Category.ID, leisure.Average)
	}

	if len(tr.Tags) != 1 || tr.Tags[0].Tag != "dulces" || tr.Tags[0].Series[0].String() != "100.00" {
		t.Errorf("tags = %+v", tr.Tags)
	}
}
