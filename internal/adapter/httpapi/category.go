package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/category"
)

// CategoryCatalog is what the HTTP layer needs from the category use cases.
type CategoryCatalog interface {
	Create(ctx context.Context, in category.CreateInput) (category.Category, error)
	List(ctx context.Context, includeArchived bool) ([]category.Category, error)
	Update(ctx context.Context, id string, in category.UpdateInput) (category.Category, error)
	ForPeriod(ctx context.Context, period string) (category.PeriodCategories, error)
	Activate(ctx context.Context, period, id string) (category.PeriodCategories, error)
	Deactivate(ctx context.Context, period, id string) (category.PeriodCategories, error)
}

// Category is a catalog entry as returned by the API.
type Category struct {
	ID        string    `json:"id" example:"market"`
	Name      string    `json:"name" example:"Mercado"`
	Group     string    `json:"group" enum:"needs,lifestyle,growth,obligations,support,other,system"`
	Committed bool      `json:"committed" doc:"Must be paid every month, whatever the amount (e.g. utilities, loans)."`
	Archived  bool      `json:"archived" doc:"Hidden from new periods but kept in history."`
	System    bool      `json:"system" doc:"Managed by the application; can't be modified."`
	CreatedAt time.Time `json:"created_at"`
}

func toCategory(c category.Category) Category {
	return Category{
		ID:        c.ID,
		Name:      c.Name,
		Group:     string(c.Group),
		Committed: c.Committed,
		Archived:  c.Archived,
		System:    c.IsSystem(),
		CreatedAt: c.CreatedAt,
	}
}

// ListCategoriesInput is the request of GET /categories.
type ListCategoriesInput struct {
	IncludeArchived bool `query:"include_archived" doc:"Also return archived categories."`
}

// CategoryListOutput is the response of GET /categories.
type CategoryListOutput struct {
	Body []Category
}

// CreateCategoryInput is the request of POST /categories.
type CreateCategoryInput struct {
	Body struct {
		Name      string `json:"name" minLength:"1" maxLength:"50" example:"Mascotas"`
		Group     string `json:"group" enum:"needs,lifestyle,growth,obligations,support,other"`
		Committed bool   `json:"committed,omitempty" doc:"Must be paid every month, whatever the amount. Defaults to false."`
	}
}

// UpdateCategoryInput is the request of PATCH /categories/{id}: only the sent fields change.
type UpdateCategoryInput struct {
	ID   string `path:"id" example:"market"`
	Body struct {
		Name      *string `json:"name,omitempty" minLength:"1" maxLength:"50"`
		Group     *string `json:"group,omitempty" enum:"needs,lifestyle,growth,obligations,support,other"`
		Committed *bool   `json:"committed,omitempty"`
		Archived  *bool   `json:"archived,omitempty" doc:"true archives the category; false restores it."`
	}
}

// CategoryOutput wraps a Category as the response body.
type CategoryOutput struct {
	Body Category
}

// PeriodCategoryInput identifies a category within a period.
type PeriodCategoryInput struct {
	PeriodParam
	ID string `path:"id" example:"travel"`
}

// PeriodCategories are the categories a period uses.
type PeriodCategories struct {
	Period        string     `json:"period" example:"2026-08"`
	Source        string     `json:"source" enum:"own,inherited,catalog" doc:"own: the period has its own list. inherited: copied from inherited_from. catalog: no earlier period is configured, so every active category applies."`
	InheritedFrom string     `json:"inherited_from,omitempty" example:"2026-07" doc:"Set only when source is inherited."`
	Categories    []Category `json:"categories"`
}

// PeriodCategoriesOutput wraps PeriodCategories as the response body.
type PeriodCategoriesOutput struct {
	Body PeriodCategories
}

func toPeriodCategories(pc category.PeriodCategories) *PeriodCategoriesOutput {
	out := PeriodCategories{
		Period:     pc.Period.String(),
		Source:     string(pc.Source),
		Categories: make([]Category, 0, len(pc.Categories)),
	}
	if pc.Source == category.SourceInherited {
		out.InheritedFrom = pc.InheritedFrom.String()
	}
	for _, c := range pc.Categories {
		out.Categories = append(out.Categories, toCategory(c))
	}
	return &PeriodCategoriesOutput{Body: out}
}

func registerCategories(api huma.API, categories CategoryCatalog) {
	huma.Register(api, huma.Operation{
		OperationID: "list-categories",
		Method:      http.MethodGet,
		Path:        "/categories",
		Summary:     "List the category catalog",
		Description: "Ordered by group (needs, lifestyle, growth, obligations, support, other, system) and name.",
		Tags:        []string{tagCategories},
	}, func(ctx context.Context, in *ListCategoriesInput) (*CategoryListOutput, error) {
		cs, err := categories.List(ctx, in.IncludeArchived)
		if err != nil {
			return nil, toHTTPError(ctx, "list categories", err)
		}
		out := &CategoryListOutput{Body: make([]Category, 0, len(cs))}
		for _, c := range cs {
			out.Body = append(out.Body, toCategory(c))
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-category",
		Method:        http.MethodPost,
		Path:          "/categories",
		Summary:       "Create a category",
		Tags:          []string{tagCategories},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *CreateCategoryInput) (*CategoryOutput, error) {
		c, err := categories.Create(ctx, category.CreateInput{
			Name:      in.Body.Name,
			Group:     in.Body.Group,
			Committed: in.Body.Committed,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "create category", err)
		}
		return &CategoryOutput{Body: toCategory(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-category",
		Method:      http.MethodPatch,
		Path:        "/categories/{id}",
		Summary:     "Update a category",
		Description: "Renames, regroups, marks as committed or archives a category. Only the sent fields change. " +
			"Categories are never deleted: archive them instead. System categories can't be modified.",
		Tags:   []string{tagCategories},
		Errors: []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *UpdateCategoryInput) (*CategoryOutput, error) {
		c, err := categories.Update(ctx, in.ID, category.UpdateInput{
			Name:      in.Body.Name,
			Group:     in.Body.Group,
			Committed: in.Body.Committed,
			Archived:  in.Body.Archived,
		})
		if err != nil {
			return nil, toHTTPError(ctx, "update category", err)
		}
		return &CategoryOutput{Body: toCategory(c)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-period-categories",
		Method:      http.MethodGet,
		Path:        "/periods/{period}/categories",
		Summary:     "List the categories of a period",
		Description: "A period without its own list inherits the one of the latest earlier configured period " +
			"(without categories archived since then); if there is none, every active catalog category applies.",
		Tags:   []string{tagCategories},
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodParam) (*PeriodCategoriesOutput, error) {
		pc, err := categories.ForPeriod(ctx, in.Period)
		if err != nil {
			return nil, toHTTPError(ctx, "list period categories", err)
		}
		return toPeriodCategories(pc), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "activate-period-category",
		Method:      http.MethodPut,
		Path:        "/periods/{period}/categories/{id}",
		Summary:     "Use a category in a period",
		Description: "Adds the category to the period, which keeps its own list from then on. Idempotent.",
		Tags:        []string{tagCategories},
		Errors:      []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodCategoryInput) (*PeriodCategoriesOutput, error) {
		pc, err := categories.Activate(ctx, in.Period, in.ID)
		if err != nil {
			return nil, toHTTPError(ctx, "activate period category", err)
		}
		return toPeriodCategories(pc), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "deactivate-period-category",
		Method:      http.MethodDelete,
		Path:        "/periods/{period}/categories/{id}",
		Summary:     "Stop using a category in a period",
		Description: "Removes the category from the period only; the category stays in the catalog and in other periods. " +
			"The period keeps its own list from then on. System categories can't be removed.",
		Tags:   []string{tagCategories},
		Errors: []int{http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *PeriodCategoryInput) (*PeriodCategoriesOutput, error) {
		pc, err := categories.Deactivate(ctx, in.Period, in.ID)
		if err != nil {
			return nil, toHTTPError(ctx, "deactivate period category", err)
		}
		return toPeriodCategories(pc), nil
	})
}
