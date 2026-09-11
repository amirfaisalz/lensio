# NusaID Development Phase Checklist & Roadmap

> Based on the NusaID KTP OCR API Product Requirements Document (PRD).  
> Designed for end-to-end API product engineering: Go, PostgreSQL, React, OpenTelemetry, Azure Container Apps, OpenTofu, and GitHub Actions.

---

## Progress Overview

- [x] **Phase 0: Project Inception, Repository Setup & AI Agent Guardrails**
- [ ] **Phase 1: Core Foundation & Scaffolding (Day 1)**
- [ ] **Phase 2: API Contract, Authentication & Security Core (Day 2)**
- [ ] **Phase 3: OCR Engine Abstraction & Processing Pipeline (Day 3)**
- [ ] **Phase 4: API Platform – Rate Limiting, Quotas & Usage Metering (Day 4)**
- [ ] **Phase 5: Developer Dashboard & Portal (Days 1, 4 & 7)**
- [ ] **Phase 6: Observability, Telemetry & Structured Logging (Day 5)**
- [ ] **Phase 7: Infrastructure as Code & Multi-Environment Provisioning (Day 5)**
- [ ] **Phase 8: CI/CD Delivery Pipeline & Automated Security Gates (Day 5 & 6)**
- [ ] **Phase 9: Reliability Engineering, Failure Drills & E2E Testing (Day 6)**
- [ ] **Phase 10: Documentation, ADRs, Demo Consumers & Portfolio Polish (Day 7)**

---

## Phase 0: Project Inception, Repository Setup & AI Agent Guardrails

Establish repository conventions, toolchains, and AI agent guardrails before writing application code.

- [ ] **Repository Conventions & Hygiene**
  - [x] Initialize Git repository with `main` branch conventions.
  - [x] Configure `.gitignore` (Go, Node/React, Terraform/OpenTofu, secrets, `.env`).
  - [x] Add `LICENSE` and standard root files.
  - [x] Set up pre-commit hooks in `.githooks/pre-commit` enforcing Typecheck, Lint, Strict Tests (100% target), and Big O / Security sanity.
- [x] **AI Agent Operational Rules (`.agents/` & `AGENTS.md`)**
  - [x] Create root `AGENTS.md` (single source of truth, core principle, MVP focus, Gemini default vision provider).
  - [x] Create `.agents/architecture.md` (domain boundaries, deep module guidelines, no leaky abstractions).
  - [x] Create `.agents/coding.md` (idiomatic Go standards, error handling, no panics, zero plaintext secrets).
  - [x] Create `.agents/testing.md` (unit test rules, synthetic data requirements, mock patterns).
  - [x] Create `.agents/security.md` (data minimization rules, PII masking, OWASP Top 10 API guidelines).
  - [x] Create `.agents/deployment.md` (immutable container tags, promotion path, rollback mandates).

---

## Phase 1: Core Foundation & Scaffolding (Day 1)

Establish the monorepo directory layout, database schema migrations, and baseline health checks.

- [ ] **Monorepo Directory Structure (`PRD Section 31`)**
  - [ ] Scaffold `apps/api/` (Go application layout: `cmd/server/main.go`, `internal/`, `migrations/`).
  - [ ] Scaffold `apps/dashboard/` (React + TypeScript + Vite/Next.js setup).
  - [ ] Scaffold `services/ocr/` (OCR engines and image preprocessing).
  - [ ] Scaffold `openapi/` (API specification directory).
  - [ ] Scaffold `tests/` (`tests/integration/`, `tests/e2e/`).
  - [ ] Scaffold `infra/` (`infra/modules/`, `infra/live/staging/`, `infra/live/production/`).
  - [ ] Scaffold `docs/` (`docs/decisions/`, `docs/incidents/`).
- [ ] **Database & Migrations (`PRD Section 21`)**
  - [ ] Set up PostgreSQL migration tool (e.g., `golang-migrate`).
  - [ ] Migration: `organizations` table (ID, name, slug, created_at, updated_at).
  - [ ] Migration: `users` table (ID, org_id, email, full_name, role, created_at).
  - [ ] Migration: `plans` table (ID, code, name, monthly_quota, rate_limit_per_minute).
  - [ ] Migration: `api_keys` table (ID, org_id, name, key_hash, prefix, scopes, environment, last_used_at, expires_at, revoked_at, created_at).
  - [ ] Migration: `ocr_requests` table (ID, org_id, api_key_id, status, confidence, latency_ms, doc_type, created_at).
  - [ ] Migration: `usage_records` table (ID, org_id, api_key_id, endpoint, status_code, latency_ms, timestamp).
  - [ ] Migration: `audit_logs` table (ID, org_id, actor_id, action, target_resource, metadata, created_at).
  - [ ] Add database seed scripts for default plans (`free`, `starter`, `pro`).
- [ ] **Liveness & Readiness Probes (`PRD Section 17`)**
  - [ ] Implement `GET /health` (process liveness probe).
  - [ ] Implement `GET /ready` (readiness probe verifying PostgreSQL ping and dependency health).
- [ ] **Local Developer Environment**
  - [ ] Create `docker-compose.yml` (PostgreSQL 16, pgAdmin / db client, Go API live-reload, dashboard).
  - [ ] Create `.env.example` with documented environment variables.
- [ ] **Acceptance Criteria**:
  - `docker compose up` starts Postgres and API cleanly.
  - `curl /health` returns HTTP 200 OK.
  - `curl /ready` returns HTTP 200 when DB is connected, and HTTP 503 when DB is stopped.

---

## Phase 2: API Contract, Authentication & Security Core (Day 2)

Build the foundational HTTP middleware, API key lifecycle, and scoped authorization.

- [ ] **OpenAPI Specification (`PRD Section 29`)**
  - [ ] Create `openapi/openapi.yaml` documenting all v1 endpoints, security schemas, error models, and examples.
  - [ ] Implement Swagger UI / scalar / OpenAPI viewer route (`/docs`, `/openapi`).
- [ ] **Standard Error Envelope & Tracing (`PRD Section 20`)**
  - [ ] Implement unified error model:
    ```json
    {
      "error": {
        "code": "invalid_document",
        "message": "The uploaded document could not be processed",
        "request_id": "req_01JABC"
      }
    }
    ```
  - [ ] Implement `RequestID` middleware (generates `req_...` or reads `X-Request-ID`, attaches to context and response header).
  - [ ] Define standardized error codes: `invalid_request`, `invalid_api_key`, `insufficient_scope`, `rate_limit_exceeded`, `quota_exceeded`, `invalid_document`, `unsupported_document`, `ocr_failed`, `low_confidence`, `internal_error`.
- [ ] **API Key Management (`PRD Section 7 & 8`)**
  - [ ] Cryptographic key generator (e.g., prefix `fg_live_` or `fg_test_` + high-entropy token).
  - [ ] Key hashing logic using SHA-256 before persistence (never store plaintext API keys).
  - [ ] Endpoint `POST /api/v1/auth/api-keys` (create key, returns plaintext token once).
  - [ ] Endpoint `GET /api/v1/auth/api-keys` (list organization's keys with masked token, prefix, scopes, last used).
  - [ ] Endpoint `DELETE /api/v1/auth/api-keys/:id` (revoke API key).
- [ ] **Authentication & Authorization Middleware**
  - [ ] API Key authentication middleware (validates `Authorization: Bearer <key>`, compares hash, verifies active status, records `last_used_at`).
  - [ ] Scope enforcement middleware (verifies key has required scopes, e.g., `ocr:read`, `ocr:write`, `usage:read`).
  - [ ] Return HTTP 401 on missing/invalid key (`invalid_api_key`), HTTP 403 on insufficient scope (`insufficient_scope`).
- [ ] **API Versioning Support (`PRD Section 6`)**
  - [ ] Route grouping `/api/v1/...` and extensible design for future `/api/v2/...`.
  - [ ] Deprecation header support (`Deprecation: true`, `Sunset: <date>`).
- [ ] **Acceptance Criteria**:
  - Unit tests covering key generation, hashing, verification, and revocation.
  - Integration tests verifying authorized requests succeed and unauthorized/out-of-scope requests receive appropriate 401/403 responses.

---

## Phase 3: OCR Engine Abstraction & Processing Pipeline (Day 3)

Implement the pluggable OCR interface, image preprocessing, extraction, and validation pipeline.

- [ ] **Pluggable OCR Architecture (`PRD Section 12`)**
  - [ ] Define Go interface:
    ```go
    type OCREngine interface {
        Extract(ctx context.Context, image []byte) (*OCRResult, error)
    }
    ```
  - [ ] Implement `MockOCREngine` for unit/integration tests with deterministic fixture responses.
  - [ ] Implement primary production engine adapter (e.g., Tesseract or Cloud Vision OCR).
- [ ] **Image Preprocessing & Validation (`PRD Section 13`)**
  - [ ] Multipart form parser for `document=<image>` (`POST /api/v1/ocr/ktp`).
  - [ ] Image format validation (JPEG, PNG, WebP) via MIME type & magic byte inspection.
  - [ ] File size enforcement (e.g., maximum 5MB).
  - [ ] Image sanitation & optional orientation normalization.
- [ ] **Document Classification & Extraction**
  - [ ] Document classifier to verify the uploaded image is an Indonesian KTP.
  - [ ] Raw OCR text extraction.
  - [ ] KTP field parsing (Regex / heuristic extraction):
    - `nik`
    - `nama`
    - `tempat_lahir`
    - `tanggal_lahir` (format `YYYY-MM-DD`)
    - `jenis_kelamin` (`LAKI-LAKI` / `PEREMPUAN`)
    - `alamat`
    - `rt_rw`
    - `kelurahan`
    - `kecamatan`
    - `agama`
    - `status_perkawinan`
    - `pekerjaan`
    - `kewarganegaraan` (`WNI` / `WNA`)
- [ ] **Field Normalization & Deterministic Validation (`PRD Section 14`)**
  - [ ] Strict NIK format validation (16 digits, numeric-only, valid province/regency code check).
  - [ ] Date of birth format and calendar validation.
  - [ ] Enum validation (religion, gender, marital status).
  - [ ] Confidence calculation per field and overall document confidence score.
  - [ ] Flag low-confidence results with `low_confidence` warning or status.
- [ ] **API Endpoints Implementation**
  - [ ] `POST /api/v1/ocr/ktp` (multipart image upload -> extraction -> structured JSON response).
  - [ ] `GET /api/v1/ocr/:id` (retrieve previous OCR execution metadata by ID).
- [ ] **Data Minimization & Privacy Rules (`PRD Section 15`)**
  - [ ] Discard temporary image buffers/files immediately after processing.
  - [ ] Never persist raw KTP images in default storage.
  - [ ] Never log PII (NIK, names, addresses) in application logs.
- [ ] **Acceptance Criteria**:
  - Test suite passes using synthetic/redacted KTP test images.
  - Invalid images return HTTP 400 with `invalid_document`.
  - Non-KTP documents return HTTP 422 with `unsupported_document`.

---

## Phase 4: API Platform – Rate Limiting, Quotas & Usage Metering (Day 4)

Implement the platform layers that turn the OCR service into a managed API product.

- [ ] **Rate Limiting Engine (`PRD Section 9`)**
  - [ ] Implement rate limiter middleware (Token Bucket / Sliding Window).
  - [ ] Tier-based rate limits based on organization's plan:
    - Free: 10 req/min
    - Starter: 30 req/min
    - Pro: 100 req/min
  - [ ] Add standard rate limit headers:
    - `X-RateLimit-Limit`
    - `X-RateLimit-Remaining`
    - `X-RateLimit-Reset`
    - `Retry-After` (on 429)
  - [ ] Return HTTP 429 with error code `rate_limit_exceeded` on violations.
- [ ] **Quota Enforcement System (`PRD Section 10 & 30`)**
  - [ ] Monthly billing period tracking per organization.
  - [ ] Quota checks before executing OCR requests:
    - Free: 100 req/month
    - Starter: 1,000 req/month
    - Pro: 10,000 req/month
  - [ ] Return HTTP 429 with error code `quota_exceeded` when monthly quota exhausted.
- [ ] **Usage Metering Subsystem (`PRD Section 11`)**
  - [ ] Non-blocking usage recorder (asynchronous dispatch / buffered channel).
  - [ ] Persist usage record per request (`request_id`, `organization_id`, `api_key_id`, `endpoint`, `status_code`, `latency_ms`, `timestamp`).
  - [ ] Implement analytics queries:
    - `GET /api/v1/usage` (summary: total requests, successes, errors, quota used/remaining).
    - `GET /api/v1/usage/daily` (daily timeseries for billing cycle).
    - `GET /api/v1/usage/endpoints` (breakdown by endpoint).
- [ ] **Account & Plan Management (`PRD Section 5`)**
  - [ ] Implement `GET /api/v1/account` (organization details, member count, active keys).
  - [ ] Implement `GET /api/v1/account/plan` (current plan, quota limits, renewal/reset date).
- [ ] **Audit Logging (`PRD Section 21`)**
  - [ ] Record audit trail for sensitive administrative events: API key creation, revocation, plan change.
- [ ] **Acceptance Criteria**:
  - Exceeding rate limit triggers HTTP 429 with expected headers.
  - Exceeding monthly quota blocks further OCR requests.
  - Every API call accurately increments usage metrics.

---

## Phase 5: Developer Dashboard & Portal (Days 1, 4 & 7)

Build a clean, responsive developer dashboard using React + TypeScript.

- [ ] **Frontend Architecture & Project Setup (`PRD Section 18`)**
  - [ ] Initialize React + TypeScript application with Tailwind CSS and Vite/Next.
  - [ ] Set up client API SDK with type generation from OpenAPI spec.
  - [ ] Implement authentication (Session / JWT / Keycloak OIDC integration).
- [ ] **Dashboard Navigation & Pages**
  - [ ] **Overview Page**:
    - High-level metric cards: Total Requests, Success Rate %, Error Rate %, P95 Latency.
    - Quota progress bar: e.g., `8,421 / 10,000 requests used`.
    - Rate limit violation counter.
    - Quick-test widget (drag & drop synthetic KTP image to test API live).
  - [ ] **API Keys Page**:
    - List active and revoked API keys (masked tokens, prefixes, scopes, last-used timestamps).
    - Create New Key modal (select scopes, name, environment; display plaintext token once).
    - Revoke API key with confirmation modal.
  - [ ] **Usage & Analytics Page**:
    - Historical usage charts (daily requests, success vs. 4xx/5xx).
    - Endpoint breakdown table.
    - Latency distribution graphs.
  - [ ] **Requests / Logs Explorer**:
    - Paginated request log table (Request ID, Timestamp, Endpoint, Status, Latency).
    - Filter by status code, date range, or API key.
    - Detail view showing latency and sanitized request metadata (NO PII).
  - [ ] **API Documentation & Quickstart**:
    - Embedded interactive OpenAPI documentation (`/docs`).
    - Copy-paste code snippets for cURL, Go, Python, and Node.js.
  - [ ] **Account & Settings Page**:
    - Organization profile, current plan details, quota limits, member list.
- [ ] **Acceptance Criteria**:
  - Developer can log in, create an API key, copy it, and see it listed.
  - Making an OCR request updates the Overview and Usage charts in real-time or on refresh.

---

## Phase 6: Observability, Telemetry & Structured Logging (Day 5)

Instrument the Go application with OpenTelemetry, Prometheus metrics, and Grafana dashboards.

- [ ] **OpenTelemetry Tracing & Metrics (`PRD Section 16`)**
  - [ ] Configure OpenTelemetry Go SDK (`go.opentelemetry.io/otel`).
  - [ ] HTTP middleware for distributed tracing (trace context propagation, trace ID in logs).
  - [ ] Database client tracing (Postgres query spans, connection pool stats).
  - [ ] Custom spans for OCR processing pipeline stages (validation, OCR engine, field extraction).
- [ ] **Core Telemetry Metrics (`PRD Section 16`)**
  - [ ] **Availability**: Request counter partitioned by route, method, and HTTP status code.
  - [ ] **Performance**: Latency histograms (P50, P95, P99) for HTTP handlers and OCR pipeline.
  - [ ] **Errors**: 4xx, 5xx, provider timeout, and fallback error counters.
  - [ ] **Rate Limiting**: 429 response counter per organization.
  - [ ] **Business Metrics**: Total OCR requests, successful extractions, low-confidence extractions, average extraction latency.
- [ ] **Structured Logging & PII Masking (`PRD Section 15`)**
  - [ ] Implement structured JSON logging (`slog` or `zap`).
  - [ ] Correlate all logs with `request_id`, `trace_id`, and `span_id`.
  - [ ] PII Sanitizer filter: strictly strip NIK, names, addresses, and images from log outputs.
- [ ] **Grafana Dashboards & Prometheus**
  - [ ] Configure Prometheus / OTel Collector endpoints.
  - [ ] Create pre-configured Grafana dashboard JSON (RED metrics: Rate, Errors, Duration).
  - [ ] Create SLA/SLO tracking panels for P95 latency (< 2s) and error budget (> 99%).
- [ ] **Acceptance Criteria**:
  - Telemetry spans are emitted during an OCR request.
  - Grafana dashboard displays real-time request counts, latency percentiles, and error rates.
  - Verified that application logs contain no sensitive identity fields.

---

## Phase 7: Infrastructure as Code & Multi-Environment Provisioning (Day 5)

Define production-grade infrastructure using OpenTofu and Terragrunt on Azure Container Apps.

- [ ] **Containerization (`PRD Section 22`)**
  - [ ] Multi-stage `Dockerfile` for Go API:
    - Builder stage using official Go image.
    - Minimal runtime stage using distroless or Alpine non-root user.
  - [ ] Multi-stage `Dockerfile` for React Dashboard (Nginx static serving / optimized build).
- [ ] **OpenTofu Modules (`infra/modules/`)**
  - [ ] Resource Group & Virtual Network module.
  - [ ] Azure Container Apps Environment module.
  - [ ] Azure Database for PostgreSQL Flexible Server module.
  - [ ] Azure Key Vault / Secret Store module.
  - [ ] Cloudflare DNS & SSL module.
- [ ] **Terragrunt Live Configurations (`infra/live/`)**
  - [ ] `infra/live/staging/`: Configuration for staging environment.
  - [ ] `infra/live/production/`: Configuration for production environment (higher redundancy, backups).
- [ ] **Security Controls & Least Privilege (`PRD Section 22 & 32`)**
  - [ ] Database credentials managed via Key Vault or managed identities.
  - [ ] Private endpoint connectivity between Container Apps and Postgres.
  - [ ] Cloudflare WAF and SSL termination rules.
- [ ] **Acceptance Criteria**:
  - `tofu plan` validates cleanly without errors.
  - Automated deployment successfully provisions staging infrastructure.

---

## Phase 8: CI/CD Delivery Pipeline & Automated Security Gates (Day 5 & 6)

Build GitHub Actions workflows with quality gates, automated scanning, and versioned promotion.

- [ ] **Pull Request Validation Workflow (`.github/workflows/ci.yml`) (`PRD Section 24`)**
  - [ ] Go unit tests with race detector (`go test -race ./...`).
  - [ ] Go code quality check (`golangci-lint run`).
  - [ ] Frontend linting and typechecking (`tsc --noEmit`, ESLint).
  - [ ] Frontend unit tests (Vitest / Jest).
  - [ ] OpenAPI contract validation (`spectral lint openapi/openapi.yaml`).
  - [ ] Integration tests against containerized Postgres in GitHub Actions runner.
- [ ] **Automated Security Scanning Suite (`PRD Section 15 & 24`)**
  - [ ] **Secret Scanning**: Gitleaks / TruffleHog action.
  - [ ] **Dependency Scanning**: `govulncheck` and `npm audit`.
  - [ ] **SAST**: `gosec` for Go source code analysis.
  - [ ] **IaC Scanning**: `tfsec` or `checkov` for OpenTofu configurations.
  - [ ] **Container Scanning**: Trivy scanning on built Docker images.
  - [ ] Quality gate: Fail pipeline if high or critical vulnerabilities are discovered.
- [ ] **Build & Image Packaging Workflow**
  - [ ] Build multi-platform Docker images.
  - [ ] Tag images with Git commit SHA and semantic version (e.g., `nusaid-api:1.4.0`).
  - [ ] Push images to GitHub Packages (GHCR) or Azure Container Registry (ACR).
- [ ] **Deployment & Promotion Workflows (`PRD Section 23`)**
  - [ ] Automated deployment to `staging` upon merge to `main`.
  - [ ] Automated smoke tests on staging environment.
  - [ ] Manual approval gate / release tag trigger for `production`.
- [ ] **Rollback Automation (`PRD Section 25`)**
  - [ ] Versioned revision deployment on Azure Container Apps.
  - [ ] Documented rollback script or GitHub Action workflow (`rollback.yml`) allowing instant traffic shift to previous stable revision (e.g., `1.4.1` -> `1.4.0`).
- [ ] **Acceptance Criteria**:
  - Pull request fails if any test, linter, or critical security scan fails.
  - Deployment creates reproducible, versioned container revisions.
  - Rollback can be triggered and completes in under 60 seconds.

---

## Phase 9: Reliability Engineering, Failure Drills & E2E Testing (Day 6)

Verify end-to-end user journeys and validate resilience through simulated production failures.

- [ ] **Playwright End-to-End Test Suite (`PRD Section 28`)**
  - [ ] Test 1: User signs in to Developer Portal.
  - [ ] Test 2: User generates an API key and copies the token.
  - [ ] Test 3: API client submits synthetic KTP image to `/api/v1/ocr/ktp`.
  - [ ] Test 4: Verify structured JSON output matches expected fields.
  - [ ] Test 5: Verify usage count increments in dashboard UI.
  - [ ] Test 6: User revokes API key; subsequent requests return 401 Unauthorized.
- [ ] **Production Failure Simulations & Drills (`PRD Section 26`)**
  - [ ] **Scenario A: OCR Provider Failure / Timeout**
    - Inject artificial latency or 500 into OCR engine.
    - Verify client receives controlled 504/502 with `ocr_failed` error code.
    - Verify provider credentials are never leaked.
  - [ ] **Scenario B: Database Outage**
    - Stop PostgreSQL container.
    - Verify `/ready` returns HTTP 503 while `/health` remains HTTP 200.
    - Verify traffic is safely rejected with controlled error handling.
  - [ ] **Scenario C: Broken Deployment Smoke Test**
    - Deploy an image that fails health check.
    - Verify staging smoke test halts and prevents promotion to production.
  - [ ] **Scenario D: Production Regression Drill**
    - Simulate error spike (> 2%).
    - Trigger alert notification.
    - Execute rollback to prior container image and verify recovery.
- [ ] **Acceptance Criteria**:
  - Playwright test passes reliably in headless CI mode.
  - Documented post-mortem drill reports for all 4 failure scenarios.

---

## Phase 10: Documentation, ADRs, Demo Consumers & Portfolio Polish (Day 7)

Finalize architectural documentation, incident reports, external demo consumers, and portfolio presentation.

- [ ] **Architecture Decision Records (`docs/decisions/`)**
  - [ ] `ADR-001`: Why Go for API and OCR service?
  - [ ] `ADR-002`: Database Schema & API Key Hashing Strategy.
  - [ ] `ADR-003`: Rate Limiting & Quota Architecture.
  - [ ] `ADR-004`: Azure Container Apps vs. Kubernetes (deliberate simplicity).
  - [ ] `ADR-005`: Data Minimization & PII Protection in OCR Pipelines.
- [ ] **Operational & Incident Runbooks (`docs/`)**
  - [ ] `docs/architecture.md` (System components, C4 diagrams, data flows).
  - [ ] `docs/security.md` (Threat model, hashing, scanning, audit trail).
  - [ ] `docs/deployment.md` (Staging, production, environment promotion).
  - [ ] `docs/rollback.md` (Step-by-step rollback procedures and verification).
  - [ ] `docs/observability.md` (Metrics, alerts, Grafana setup, SLO definitions).
  - [ ] `docs/incidents/` (Post-mortems from Phase 9 failure drills).
- [ ] **Demo Consumers (`PRD Section 3 & Section 36`)**
  - [ ] `VeriForm` demo: Lightweight synthetic identity onboarding script/app consuming NusaID API.
  - [ ] `RentEase` demo: Vehicle rental verification script consuming NusaID API.
  - [ ] Verify both consumers run as independent external clients.
- [ ] **Root `README.md` Polish (`PRD Section 37 & 39`)**
  - [ ] Badges: Build status, Go Report Card, coverage, license, security scans.
  - [ ] One-sentence mission statement: *"NusaID is a developer-first KTP OCR API that lets applications extract structured Indonesian KTP data through a secure, rate-limited, observable, and production-ready API."*
  - [ ] Architecture diagram (ASCII / Mermaid / SVG).
  - [ ] Quickstart guide (cURL commands, local docker-compose setup).
  - [ ] DORA metrics and production engineering highlights table (`PRD Section 38`).
- [ ] **Demo Video / Walkthrough Recording**
  - [ ] Record 3-5 minute demo video demonstrating the full lifecycle:
    1. Account login & API key creation.
    2. cURL OCR request with synthetic KTP image.
    3. Structured JSON response.
    4. Real-time usage and quota update in dashboard.
    5. Rate limit trigger (HTTP 429).
    6. Deployment promotion & rollback demonstration.

---

## Definition of Done (DoD) Checklist (`PRD Section 35`)

Before tagging `v1.0.0`, all items below must be verified:

- [ ] A developer can create an account and obtain an API key.
- [ ] Plaintext API keys are never stored in the database.
- [ ] Developer can submit a synthetic KTP image to `POST /api/v1/ocr/ktp`.
- [ ] API returns structured, normalized JSON data.
- [ ] Invalid/unsupported documents return predictable, documented error codes.
- [ ] Rate limits and monthly quotas are actively enforced with standard headers.
- [ ] Usage metering records every API invocation accurately.
- [ ] Application logs are strictly free of PII (NIK, names, addresses, raw images).
- [ ] `/health` and `/ready` endpoints respond accurately to process and DB states.
- [ ] OpenTelemetry emits traces and metrics to Grafana.
- [ ] CI pipeline validates unit tests, linters, and security scanners.
- [ ] Playwright E2E tests run and pass automatically in CI.
- [ ] Infrastructure is fully codified using OpenTofu & Terragrunt.
- [ ] Staging and Production deployments function via automated workflows.
- [ ] Deployment rollback can be executed reliably in under 60 seconds.
- [ ] Failure drills (OCR failure, DB outage, smoke test failure, error regression) are documented.
- [ ] At least one external demo consumer (`VeriForm` or `RentEase`) successfully interacts with the API.
- [ ] All architectural documentation, ADRs, and README are complete.

---

## Future Product Roadmap (Post-MVP / Next Iterations)

> [!NOTE]
> **Current Strategic Focus**: MVP scope is 100% focused on **Indonesian KTP OCR** (`POST /api/v1/ocr/ktp`) and establishing the end-to-end API product platform.
> The product lines below are captured for subsequent development phases after the KTP OCR platform is in production.

```text
NusaID Product Ecosystem
│
├── Document OCR Expansion
│   ├── [x] KTP OCR (Current Focus - Active MVP)
│   ├── [ ] SIM OCR (Surat Izin Mengemudi / Driver's License)
│   ├── [ ] Passport OCR (Indonesian & International Passports)
│   ├── [ ] NPWP OCR (Nomor Pokok Wajib Pajak / Tax ID)
│   ├── [ ] KK OCR (Kartu Keluarga / Family Register)
│   └── [ ] Invoice OCR (Commercial Invoices & E-Faktur)
│
├── Verification & Trust Services
│   ├── [ ] Document Verification (Tamper detection, hologram check, synthetic forgery detection)
│   └── [ ] Identity Verification (Liveness check, selfie-to-KTP facial comparison)
│
└── Intelligent Extraction
    └── [ ] AI Document Extraction (Flexible vision-based parsing for custom / semi-structured documents)
```

