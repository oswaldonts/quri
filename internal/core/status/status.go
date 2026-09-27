// Package status tells whether an entry already happened or is only expected.
package status

import "fmt"

// Status of an income, expense or exchange.
type Status string

const (
	// Planned entries are expected but haven't happened yet; they only count in
	// projected views.
	Planned Status = "planned"
	// Confirmed entries happened. It is the default.
	Confirmed Status = "confirmed"
)

// Parse reads a status; an empty string means Confirmed.
func Parse(s string) (Status, error) {
	switch Status(s) {
	case "", Confirmed:
		return Confirmed, nil
	case Planned:
		return Planned, nil
	default:
		return "", fmt.Errorf("status %q (want %q or %q)", s, Planned, Confirmed)
	}
}
