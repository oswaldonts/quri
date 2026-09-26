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
}
