// Package openapi holds the OpenAPI 3.1 document of Mavio's Connect
// services and its plain HTTP media routes, generated from the contracts
// and base.yaml by pnpm nx run proto:generate.
package openapi

import _ "embed"

// Spec is the OpenAPI document, as JSON.
//
//go:embed gen/mavio.openapi.json
var Spec []byte
