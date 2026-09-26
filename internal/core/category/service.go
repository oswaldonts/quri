package category

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"
)

// Repository is the port the service needs to persist the catalog.
type Repository interface {
	// Create returns ErrNameTaken if the name is already in use.
	Create(ctx context.Context, c Category) error
	// Get returns ErrNotFound if no category has the ID.
	Get(ctx context.Context, id string) (Category, error)
	// List returns the catalog ordered by group and name.
	List(ctx context.Context, includeArchived bool) ([]Category, error)
	// Update replaces the stored category; it returns ErrNameTaken or ErrNotFound.
	Update(ctx context.Context, c Category) error
}

// Service holds the catalog use cases.
type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() string
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, newID: rand.Text}
}

// CreateInput carries a new category.
type CreateInput struct {
	Name      string
	Group     string
	Committed bool
}

// Create adds a category to the catalog.
func (s *Service) Create(ctx context.Context, in CreateInput) (Category, error) {
	name, err := parseName(in.Name)
	if err != nil {
		return Category{}, invalid(err)
	}
	group, err := parseGroup(in.Group)
	if err != nil {
		return Category{}, invalid(err)
	}

	c := Category{
		ID:        s.newID(),
		Name:      name,
		Group:     group,
		Committed: in.Committed,
		CreatedAt: s.now().UTC(),
	}
	if err := s.repo.Create(ctx, c); err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}
	return c, nil
}

// List returns the catalog; archived categories only if includeArchived is true.
func (s *Service) List(ctx context.Context, includeArchived bool) ([]Category, error) {
	return s.repo.List(ctx, includeArchived)
}

// UpdateInput is a partial update: nil fields are left unchanged.
type UpdateInput struct {
	Name      *string
	Group     *string
	Committed *bool
	Archived  *bool
}

// Update changes the given fields of a category. System categories can't be modified.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (Category, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return Category{}, err
	}
	if c.IsSystem() {
		return Category{}, fmt.Errorf("%w: %s", ErrSystemCategory, c.Name)
	}

	if in.Name != nil {
		if c.Name, err = parseName(*in.Name); err != nil {
			return Category{}, invalid(err)
		}
	}
	if in.Group != nil {
		if c.Group, err = parseGroup(*in.Group); err != nil {
			return Category{}, invalid(err)
		}
	}
	if in.Committed != nil {
		c.Committed = *in.Committed
	}
	if in.Archived != nil {
		c.Archived = *in.Archived
	}

	if err := s.repo.Update(ctx, c); err != nil {
		return Category{}, fmt.Errorf("update category: %w", err)
	}
	return c, nil
}

func invalid(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}
