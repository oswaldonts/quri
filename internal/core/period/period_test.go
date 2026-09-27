package period

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "2026-07", want: "2026-07"},
		{in: "2026-12", want: "2026-12"},
		{in: "2026-7", wantErr: true},
		{in: "2026-13", wantErr: true},
		{in: "2026-00", wantErr: true},
		{in: "26-07", wantErr: true},
		{in: "2026-07-01", wantErr: true},
		{in: "julio", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidPeriod) {
					t.Fatalf("err = %v, want %v", err, ErrInvalidPeriod)
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

func TestSpendMonth(t *testing.T) {
	tests := []struct {
		earned string
		spend  string
	}{
		{earned: "2026-07", spend: "2026-08"},
		{earned: "2026-12", spend: "2027-01"},
	}
	for _, tt := range tests {
		t.Run(tt.earned, func(t *testing.T) {
			p, err := Parse(tt.earned)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := p.SpendMonth().String(); got != tt.spend {
				t.Errorf("SpendMonth() = %s, want %s", got, tt.spend)
			}
		})
	}
}

func TestNextCompareMonthsBetween(t *testing.T) {
	dec, _ := Parse("2026-12")
	jan, _ := Parse("2027-01")
	jul, _ := Parse("2026-07")

	if dec.Next() != jan {
		t.Errorf("2026-12 Next = %s, want 2027-01", dec.Next())
	}
	if jul.Compare(dec) != -1 || dec.Compare(jul) != 1 || jan.Compare(jan) != 0 || dec.Compare(jan) != -1 {
		t.Error("Compare is wrong")
	}
	if n := jul.MonthsBetween(jan); n != 7 {
		t.Errorf("2026-07..2027-01 = %d months, want 7", n)
	}
	if n := jan.MonthsBetween(jul); n != 0 {
		t.Errorf("reversed range = %d, want 0", n)
	}
}
