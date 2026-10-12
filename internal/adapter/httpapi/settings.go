package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/oswaldoaquinas/go-income-mgr/internal/core/period"
	"github.com/oswaldoaquinas/go-income-mgr/internal/core/settings"
)

// SettingsService is what the HTTP layer needs from the settings use cases.
type SettingsService interface {
	Get(ctx context.Context) (settings.Settings, error)
	Update(ctx context.Context, in settings.UpdateInput) (settings.Settings, error)
	SpendOffset(ctx context.Context) (int, error)
}

// Settings are the preferences of the installation.
type Settings struct {
	SpendOffset     int    `json:"spend_offset" minimum:"0" maximum:"1" doc:"Months between earning money and spending it: 0 spends it the same month, 1 the following one."`
	DefaultCurrency string `json:"default_currency" example:"USD" doc:"Currency the app opens in. Empty: the first one in the currency list."`
}

func toSettings(s settings.Settings) Settings {
	return Settings{SpendOffset: s.SpendOffset, DefaultCurrency: s.DefaultCurrency}
}

// SettingsOutput wraps Settings as the response body.
type SettingsOutput struct {
	Body Settings
}

// UpdateSettingsInput is the request of PATCH /settings: only the sent fields change.
type UpdateSettingsInput struct {
	Body struct {
		SpendOffset     *int    `json:"spend_offset,omitempty" minimum:"0" maximum:"1"`
		DefaultCurrency *string `json:"default_currency,omitempty" pattern:"^([A-Za-z]{3})?$" doc:"A registered currency; empty clears it."`
	}
}

// spendMonth is the month in which the money of p is spent.
func spendMonth(p period.Period, offset int) string { return p.Shift(offset).String() }

func registerSettings(api huma.API, svc SettingsService) {
	huma.Register(api, huma.Operation{
		OperationID: "get-settings",
		Method:      http.MethodGet,
		Path:        "/settings",
		Summary:     "Get the settings",
		Tags:        []string{tagSettings},
	}, func(ctx context.Context, _ *struct{}) (*SettingsOutput, error) {
		s, err := svc.Get(ctx)
		if err != nil {
			return nil, toHTTPError(ctx, "get settings", err)
		}
		return &SettingsOutput{Body: toSettings(s)}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-settings",
		Method:      http.MethodPatch,
		Path:        "/settings",
		Summary:     "Change the settings",
		Description: "Changing spend_offset only changes which month each period's money is said to be spent in; " +
			"nothing stored is modified.",
		Tags:   []string{tagSettings},
		Errors: []int{http.StatusUnprocessableEntity},
	}, func(ctx context.Context, in *UpdateSettingsInput) (*SettingsOutput, error) {
		s, err := svc.Update(ctx, settings.UpdateInput{SpendOffset: in.Body.SpendOffset, DefaultCurrency: in.Body.DefaultCurrency})
		if err != nil {
			return nil, toHTTPError(ctx, "update settings", err)
		}
		return &SettingsOutput{Body: toSettings(s)}, nil
	})
}
