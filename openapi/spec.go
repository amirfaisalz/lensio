package openapi

import _ "embed"

// Spec contains the embedded OpenAPI 3.0 YAML specification.
//
//go:embed openapi.yaml
var Spec []byte
