# Architecture Guidelines

> Canonical rules for system design, service boundaries, and module architecture in Lensio.

---

## 1. Monorepo Boundaries

The repository follows a clean, decoupled monorepo structure:

```text
Lensio/
├── apps/
│   ├── api/             # Go REST API service
│   │   ├── cmd/server/  # main.go entrypoint
│   │   ├── internal/    # Private application packages
│   │   └── migrations/  # SQL migration files
│   └── dashboard/       # React + TypeScript Developer Portal
├── services/
│   └── ocr/             # OCR Engine interfaces & provider adapters
├── openapi/             # OpenAPI 3.0 specification contract
├── tests/
│   ├── integration/     # Go integration tests
│   └── e2e/             # Playwright browser end-to-end tests
├── infra/               # OpenTofu & Terragrunt IaC modules
├── docs/                # Architecture docs, ADRs, and incident reports
└── .agents/             # AI agent operational guidelines
```

---

## 2. Go Backend Architecture (`apps/api`)

Follow the standard Go project layout with distinct architectural layers:

```text
HTTP Request
     │
     ▼
[Transport Layer]       (cmd/server, internal/http/router.go, middleware/)
     │                   - Request ID injection, CORS, Auth, Rate Limit, Tracing
     ▼
[Handler Layer]         (internal/http/handlers/)
     │                   - Input parsing, multipart decoding, response rendering
     ▼
[Domain / Service Layer](internal/service/)
     │                   - Business rules, quota evaluation, usage metering dispatch
     ▼
┌───────────────────┐    ┌─────────────────────────┐
│ Repository Layer  │    │ OCR Service Abstraction │
│ (internal/store/) │    │ (services/ocr/)         │
└─────────┬─────────┘    └────────────┬────────────┘
          ▼                           ▼
     PostgreSQL               Gemini / Mock Engine
```

### Layer Constraints
1. **Unidirectional Flow**: Transport depends on Handlers -> Handlers depend on Services -> Services depend on Repositories and OCR Engine. Never reverse this dependency.
2. **Context Propagation**: Always pass `ctx context.Context` through all layers for tracing and cancellation.
3. **No Database Leaks**: Never pass database-specific structs (e.g. `pgx.Rows`, `sql.Result`) into the HTTP handler or presentation layer.

---

## 3. Pluggable OCR Engine (`services/ocr`)

The OCR processing engine must be strictly isolated behind the `OCREngine` interface:

```go
package ocr

import "context"

type OCREngine interface {
    Extract(ctx context.Context, image []byte) (*OCRResult, error)
}
```

### Rules
- The HTTP layer interacts exclusively with `OCREngine`. It must never import or call provider-specific SDKs directly.
- All provider-specific implementations (`gemini.go`, `mock.go`, etc.) live in `services/ocr/providers/`.
- Provider selection is controlled via environment configuration (`OCR_PROVIDER=gemini_flash` or `OCR_PROVIDER=mock`).

---

## 4. Health & Liveness Design

Separate process health from dependency availability:
- **Liveness (`GET /health`)**: Verifies the Go web server process is alive and responsive. Does not probe external databases.
- **Readiness (`GET /ready`)**: Verifies all critical dependencies (PostgreSQL connection pool) are healthy. Returns HTTP 503 if the database is unreachable.

---

## 5. Architectural Decision Records (ADRs)

Any significant change to database technology, authentication flows, rate limiting algorithms, or infrastructure topology requires an ADR in `docs/decisions/ADR-xxx.md`.
