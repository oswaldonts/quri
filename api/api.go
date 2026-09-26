// Package api holds the OpenAPI specification, embedded into the binary.
package api

import _ "embed"

// OpenAPI is the content of openapi.yaml, read at compile time.
//
//go:embed openapi.yaml
var OpenAPI []byte
