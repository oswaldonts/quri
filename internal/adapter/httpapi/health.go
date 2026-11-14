package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// HealthOutput is the response of GET /health.
type HealthOutput struct {
	Body struct {
		Status       string `json:"status" example:"ok" doc:"Always \"ok\" when the service is up."`
		AuthRequired bool   `json:"auth_required" doc:"false when the app runs without a login (the desktop build)."`
	}
}

func registerHealth(api huma.API, authRequired bool) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
		Tags:        []string{tagSystem},
	}, func(_ context.Context, _ *struct{}) (*HealthOutput, error) {
		out := &HealthOutput{}
		out.Body.Status = "ok"
		out.Body.AuthRequired = authRequired
		return out, nil
	})
}
