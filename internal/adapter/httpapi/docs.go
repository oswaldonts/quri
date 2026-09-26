package httpapi

import (
	"net/http"

	"github.com/oswaldoaquinas/go-income-mgr/api"
)

// scalarPage renders the Scalar API reference for /openapi.yaml.
// The script is pinned to a version so the docs don't change unexpectedly.
const scalarPage = `<!doctype html>
<html>
  <head>
    <title>go-income-mgr API</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>
  <body>
    <div id="app"></div>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.72.1"></script>
    <script>
      Scalar.createApiReference('#app', { url: '/openapi.yaml' })
    </script>
  </body>
</html>
`

// openAPISpec handles GET /openapi.yaml.
func openAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(api.OpenAPI)
}

// docs handles GET /docs.
func docs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(scalarPage))
}
