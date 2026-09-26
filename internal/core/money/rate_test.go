package money

import (
	"errors"
	"testing"
)

func TestParseRate(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "3.25", want: "3.2500"},
		{in: "3.2567", want: "3.2567"},
		{in: "4", want: "4.0000"},
		{in: "0.2667", want: "0.2667"},
		{in: "3.25671", wantErr: true},
		{in: "0", wantErr: true},
		{in: "-3.25", wantErr: true},
		{in: "abc", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseRate(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidRate) {
					t.Fatalf("err = %v, want %v", err, ErrInvalidRate)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name   string
		amount string
		from   Currency
		rate   string
		to     Currency
		want   string
	}{
		{"exact", "1096.00", USD, "3.25", PEN, "3562.00"},
		{"truncates, never rounds up", "1096.37", USD, "3.2567", PEN, "3570.54"}, // 3570.548179
		{"sub-cent result truncates to zero", "0.01", USD, "0.5", PEN, "0.00"},
		{"reverse direction", "100.00", PEN, "0.2667", USD, "26.67"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount, err := Parse(tt.amount, tt.from)
			if err != nil {
				t.Fatalf("parse amount: %v", err)
			}
			rate, err := ParseRate(tt.rate)
			if err != nil {
				t.Fatalf("parse rate: %v", err)
			}

			got, err := amount.Convert(rate, tt.to)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Currency() != tt.to || got.String() != tt.want {
				t.Errorf("got %s %s, want %s %s", got, got.Currency(), tt.want, tt.to)
			}
		})
	}
}

func TestConvertInvalid(t *testing.T) {
	amount, _ := Parse("10", USD)
	rate, _ := ParseRate("3.25")

	if _, err := amount.Convert(rate, Currency("EUR")); !errors.Is(err, ErrUnsupportedCurrency) {
		t.Errorf("unsupported target: err = %v, want %v", err, ErrUnsupportedCurrency)
	}
	if _, err := amount.Convert(Rate{}, PEN); !errors.Is(err, ErrInvalidRate) {
		t.Errorf("zero rate: err = %v, want %v", err, ErrInvalidRate)
	}

	huge, _ := Parse("9999999999999.99", USD)
	maxRate, _ := ParseRate("9999.9999")
	if _, err := huge.Convert(maxRate, PEN); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("out of range: err = %v, want %v", err, ErrInvalidAmount)
	}
}

func TestNewRate(t *testing.T) {
	r, err := NewRate(32500)
	if err != nil || r.String() != "3.2500" || r.Scaled() != 32500 {
		t.Errorf("NewRate(32500) = %s (scaled %d), %v; want 3.2500", r, r.Scaled(), err)
	}
	if _, err := NewRate(0); !errors.Is(err, ErrInvalidRate) {
		t.Errorf("NewRate(0): err = %v, want %v", err, ErrInvalidRate)
	}
}
