package status

import "testing"

func TestParse(t *testing.T) {
	for in, want := range map[string]Status{"": Confirmed, "confirmed": Confirmed, "planned": Planned} {
		if got, err := Parse(in); err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := Parse("pending"); err == nil {
		t.Error(`Parse("pending") must fail`)
	}
}
