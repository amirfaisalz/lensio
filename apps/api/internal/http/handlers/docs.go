package handlers

import (
	"fmt"
	"net/http"

	"github.com/amirfaisalz/nusaid/openapi"
)

// OpenAPIHandler serves the raw OpenAPI YAML contract.
func OpenAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(openapi.Spec)
	}
}

// DocsHandler serves an embedded, interactive Scalar API documentation UI.
func DocsHandler(specPath string) http.HandlerFunc {
	html := fmt.Sprintf(`<!doctype html>
<html>
  <head>
    <title>NusaID API Documentation</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🇮🇩</text></svg>">
    <style>
      body {
        margin: 0;
        padding: 0;
      }
    </style>
  </head>
  <body>
    <script id="api-reference" data-url="%s"></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`, specPath)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}
}
