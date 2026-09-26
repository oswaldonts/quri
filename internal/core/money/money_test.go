package money

import (
	"errors"
	"slices"
	"testing"
)

func TestParseCurrency(t *testing.T) {
	tests := []struct {
		in      string
		want    Currency
		wantErr error
	}{
		{in: "USD", want: USD},
		{in: "pen", want: PEN},
		{in: " usd ", want: USD},
		{in: "$", wantErr: ErrUnsupportedCurrency},
		{in: "ars", want: ARS},
		{in: "EUR", wantErr: ErrUnsupportedCurrency},
		{in: "", wantErr: ErrUnsupportedCurrency},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseCurrency(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		in        string
		wantMinor int64
		wantStr   string
		wantErr   error
	}{
		{in: "1330", wantMinor: 133000, wantStr: "1330.00"},
		{in: "1330.00", wantMinor: 133000, wantStr: "1330.00"},
		{in: "142.5", wantMinor: 14250, wantStr: "142.50"},
		{in: "0.05", wantMinor: 5, wantStr: "0.05"},
		{in: "-7", wantMinor: -700, wantStr: "-7.00"},
		{in: "0", wantMinor: 0, wantStr: "0.00"},
		{in: "1.005", wantErr: ErrInvalidAmount},
		{in: "", wantErr: ErrInvalidAmount},
		{in: "abc", wantErr: ErrInvalidAmount},
		{in: "12.", wantErr: ErrInvalidAmount},
		{in: ".5", wantErr: ErrInvalidAmount},
		{in: "1,330.00", wantErr: ErrInvalidAmount},
		{in: "+5", wantErr: ErrInvalidAmount},
		{in: "99999999999999", wantErr: ErrInvalidAmount},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in, USD)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Amount() != tt.wantMinor {
				t.Errorf("Amount() = %d, want %d", got.Amount(), tt.wantMinor)
			}
			if got.String() != tt.wantStr {
				t.Errorf("String() = %q, want %q", got.String(), tt.wantStr)
			}
		})
	}
}

func TestParseUnsupportedCurrency(t *testing.T) {
	if _, err := Parse("10", Currency("EUR")); !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("err = %v, want %v", err, ErrUnsupportedCurrency)
	}
}

func TestAdd(t *testing.T) {
	a, _ := Parse("0.10", USD)
	b, _ := Parse("0.20", USD)

	sum, err := a.Add(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.String() != "0.30" {
		t.Errorf("0.10 + 0.20 = %s, want 0.30", sum)
	}

	pen, _ := Parse("1", PEN)
	if _, err := a.Add(pen); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("err = %v, want %v", err, ErrCurrencyMismatch)
	}
}

func TestCurrencies(t *testing.T) {
	want := []Currency{ARS, COP, PEN, USD, VES}
	if got := Currencies(); !slices.Equal(got, want) {
		t.Errorf("Currencies() = %v, want %v (sorted)", got, want)
	}
}
