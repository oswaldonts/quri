// Package category manages the global catalog of expense categories.
//
// A category exists once in the catalog; each period later decides which
// categories it uses, so the same category can be compared across months.
// Categories are never deleted, only archived.
package category

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrInvalidInput wraps every validation error.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means no category has the given ID.
	ErrNotFound = errors.New("category not found")
	// ErrNameTaken means another category already uses the name (case-insensitive).
	ErrNameTaken = errors.New("category name already in use")
	// ErrSystemCategory means the category is managed by the system and can't be modified.
	ErrSystemCategory = errors.New("system category can't be modified")
)

// Group is a high-level bucket of categories, used to read the dashboard at two levels.
type Group string

const (
	GroupNeeds       Group = "needs"
	GroupLifestyle   Group = "lifestyle"
	GroupGrowth      Group = "growth"
	GroupObligations Group = "obligations"
	GroupSupport     Group = "support"
	GroupOther       Group = "other"
	// GroupSystem holds categories the application manages itself (e.g. exchanges).
	GroupSystem Group = "system"
)

// UserGroups are the groups a user may assign; GroupSystem is reserved.
var UserGroups = []Group{GroupNeeds, GroupLifestyle, GroupGrowth, GroupObligations, GroupSupport, GroupOther}

func parseGroup(s string) (Group, error) {
	for _, g := range UserGroups {
		if Group(s) == g {
			return g, nil
		}
	}
	return "", fmt.Errorf("group %q is not valid", s)
}

const maxNameLength = 50

// Category is an entry of the catalog.
type Category struct {
	ID        string
	Name      string
	Group     Group
	Committed bool // must be paid every month, whatever the amount (e.g. utilities, loans)
	Archived  bool // hidden from new periods but kept in history
	CreatedAt time.Time
}

// IsSystem reports whether the category is managed by the application.
func (c Category) IsSystem() bool { return c.Group == GroupSystem }

// NameKey is the normalized name used to detect duplicates: "Mercado" and " mercado " clash.
func NameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func parseName(s string) (string, error) {
	name := strings.TrimSpace(s)
	if name == "" {
		return "", errors.New("name is required")
	}
	if n := len([]rune(name)); n > maxNameLength {
		return "", fmt.Errorf("name longer than %d characters", maxNameLength)
	}
	return name, nil
}
