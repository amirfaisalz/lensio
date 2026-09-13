# Lensio Development Tracking & Execution Matrix

> **Target Role**: Senior Full-Stack Engineer, Platform & API  
> **Core Architecture**: Go, PostgreSQL, React + TypeScript, Keycloak OIDC, SpiceDB, OpenTelemetry, Azure Container Apps, OpenTofu, GitHub Actions.  
> **Strategic Positioning**: *A production-grade Identity Document OCR API platform.*

---

## Master Progress Overview

### Completed Phases (Phases 0 – 11)
- [x] **Phase 0: Project Inception, Setup & Guardrails** — Repository conventions, `.githooks/pre-commit` (4-layer verification), root `AGENTS.md`, and operational guardrails (`.agents/`).
- [x] **Phase 1: Core Foundation & Scaffolding** — Go monorepo layout, PostgreSQL schema migrations (`golang-migrate`), `/health` & `/ready` probes, Docker Compose.
- [x] **Phase 2: API Contract & Security Core** — OpenAPI 3.0 spec, SHA-256 API key hashing lifecycle, scoped middleware (`ocr:read`, `ocr:write`, `usage:read`), standardized error envelopes (`request_id`).
- [x] **Phase 3: OCR Engine & KTP Pipeline** — Pluggable `OCREngine` interface, synthetic `MockEngine`, Gemini Flash vision engine adapter, 16-digit NIK validator, confidence scoring, zero-PII data minimization.
- [x] **Phase 4: Platform Engine** — O(1) in-memory token bucket rate limiter, monthly plan quota enforcement, non-blocking asynchronous usage metering, analytics endpoints, audit logging.
- [x] **Phase 5: Developer Dashboard** — React + TypeScript + Tailwind portal: real-time metric cards, quota progress, interactive OCR playground, API key lifecycle manager, request log explorer.
- [x] **Phase 6: Observability & Telemetry** — OpenTelemetry Go SDK instrumentation, HTTP & DB tracing spans, Prometheus RED metrics (`/metrics`), Grafana dashboard provisioning, structured `slog` with PII sanitization.
- [x] **Phase 7: Infrastructure as Code** — Multi-stage Dockerfiles, OpenTofu modules (`infra/modules/`) for Azure Container Apps, PostgreSQL Flexible Server, Azure Key Vault, Cloudflare WAF/TLS, Terragrunt staging/production configs.
- [x] **Phase 8: CI/CD & Delivery Gates** — Automated GitHub Actions CI: Go race detector, golangci-lint, Biome/TypeScript check, Spectral OpenAPI linter, Gitleaks, govulncheck, gosec SAST, Trivy container scan, sub-60s automated rollback scripts.
- [x] **Phase 9: Reliability & Failure Drills** — Playwright E2E test suite, and 4 documented production failure drill post-mortems (upstream timeout, DB outage, broken smoke test gate, production regression).
- [x] **Phase 10: Documentation & Portfolio Polish** — ADRs (001–005), production runbooks (`architecture.md`, `security.md`, `observability.md`, `rollback.md`), external client demos (`VeriForm`, `RentEase`), open-source README.
- [x] **Phase 11: Production Platform Maturity & Enterprise Alignment** — k6 load testing (1,000 rps), API idempotency, Keycloak OIDC, SpiceDB ReBAC, circuit breaker resilience, Prometheus SLO alerting, ADRs (006–007), and SIM OCR (`POST /api/v1/ocr/sim`).

---

## Active Phase: Phase 12 — Document OCR Expansion

> [!NOTE]
> All core platform foundation, reliability, and security hardening (Phases 0–11) are complete.
> Phase 12 focuses on expanding document support horizontally according to the product roadmap.

### Document Expansion Candidates
- [x] **Phase 12.1: Indonesian Passport OCR (`POST /api/v1/ocr/passport`)**
  - [x] Domain validator: MRZ (Machine Readable Zone) checksum & 9-character passport number validation.
  - [x] OCR prompt & structured parsing: Full name, passport number, nationality, date of birth, sex, expiry date, issuing office.
  - [x] API endpoint & handler with idempotency, quota check, and usage metering.
  - [x] Synthetic fixtures & unit/integration tests with 100% test coverage.
  - [x] OpenAPI 3.0 specification update & dashboard playground integration.
- [x] **Phase 12.2: Indonesian NPWP OCR (`POST /api/v1/ocr/npwp`)**
  - [x] Domain validator: 15-digit / 16-digit NPWP format and Luhn/checksum structure.
  - [x] OCR extraction: NPWP number, taxpayer name, NIK link, address, tax office (KPP).
  - [x] Endpoint integration, synthetic fixtures, and automated test suite.
  - [x] OpenAPI 3.0 specification update & dashboard playground integration.
- [x] **Phase 12.3: Indonesian KK (Kartu Keluarga) OCR (`POST /api/v1/ocr/kk`)**
  - [x] Domain validator: 16-digit Nomor KK and family member table extraction.
  - [x] Multi-record tabular parsing & relationship mapping.
  - [x] API endpoint & handler with idempotency, quota check, and usage metering.
  - [x] Synthetic fixtures & unit/integration tests with 100% test coverage.
  - [x] OpenAPI 3.0 specification update & dashboard playground integration.
- [ ] **Phase 12.4: Indonesian Commercial Invoice / E-Faktur OCR (`POST /api/v1/ocr/invoice`)**
  - [ ] Line-item extraction, tax calculation validation, and supplier details.

---

## Standard Implementation Checklist for New OCR Documents

Each new document type introduced in Phase 12 must strictly follow our production architecture standards:

1. **Deterministic Validator First (`services/ocr/<doc>_validator.go`)**:
   - Algorithmically validate document numbers (checksum, length, regex) before trusting LLM output.
   - Comprehensive unit tests with negative/positive synthetic test cases.
2. **Vision Engine Extraction & Parser (`services/ocr/`)**:
   - Add structured DTO to `services/ocr/engine.go`.
   - Update `GeminiEngine` system prompt and JSON schema extraction.
   - Update `MockEngine` with deterministic fixtures in `tests/fixtures/synthetic/`.
3. **API Layer (`apps/api/internal/http/handlers/ocr.go`)**:
   - Implement `POST /api/v1/ocr/<doc>` handler.
   - Attach middleware chain: Authentication, Scopes (`ocr:write`), Idempotency (`Idempotency-Key`), and Quota enforcement.
4. **Contract & Documentation (`openapi/openapi.yaml`)**:
   - Define request multipart schema, success response DTO, and error states.
5. **Dashboard Playground (`apps/dashboard/`)**:
   - Add document tab/selector in Live Playground.
   - Render document-specific fields and confidence badges.
6. **Zero-PII & Quality Assurance**:
   - In-memory processing only (never write document images or PII to disk or logs).
   - Pass all 4 pre-commit layers: `go test -race -cover ./...`, `golangci-lint`, and dashboard typecheck.
