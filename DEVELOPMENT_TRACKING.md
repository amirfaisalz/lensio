# Lensio Development Tracking & Execution Matrix

> **Target Role**: Senior Full-Stack Engineer, Platform & API  
> **Core Architecture**: Go, PostgreSQL, React + TypeScript, Keycloak OIDC, SpiceDB, OpenTelemetry, Azure Container Apps, OpenTofu, GitHub Actions.  
> **Strategic Positioning**: *A production-style API platform, using KTP OCR as the business use case.*

---

## Master Progress Overview

### Completed Foundation (Phases 0 – 10)
- [x] **Phase 0: Project Inception, Repository Setup & AI Agent Guardrails**
- [x] **Phase 1: Core Foundation & Scaffolding**
- [x] **Phase 2: API Contract, Authentication & Security Core**
- [x] **Phase 3: OCR Engine Abstraction & Processing Pipeline**
- [x] **Phase 4: API Platform – Rate Limiting, Quotas & Usage Metering**
- [x] **Phase 5: Developer Dashboard & Portal**
- [x] **Phase 6: Observability, Telemetry & Structured Logging**
- [x] **Phase 7: Infrastructure as Code & Multi-Environment Provisioning**
- [x] **Phase 8: CI/CD Delivery Pipeline & Automated Security Gates**
- [x] **Phase 9: Reliability Engineering, Failure Drills & E2E Testing**
- [x] **Phase 10: Documentation, ADRs, Demo Consumers & Portfolio Polish**

### Active V2 Platform Maturity & Enterprise Alignment (Phase 11)
- [x] **Phase 11.1: Real Load Testing & RED Metrics Analysis (`k6`)**
- [x] **Phase 11.2: API Idempotency Subsystem (`Idempotency-Key`)**
- [x] **Phase 11.3: Verifiable Deployment & Rollback Evidence Log**
- [x] **Phase 11.4: Centralized Human Identity (Keycloak + OIDC / JWT)**
- [x] **Phase 11.5: Fine-Grained Authorization (SpiceDB / ReBAC)**
- [x] **Phase 11.6: Deep Incident Engineering & Circuit Breaker Code Fix**
- [x] **Phase 11.7: Production SLOs & Actionable Alerting Rules**
- [ ] **Phase 11.8: Architectural Decision Record: Distributed vs Local Rate Limiting**
- [ ] **Phase 11.9: API Versioning Strategy & Contract Evolution**

---

## Section I: Completed Historical Phases (Summary)

### [x] Phase 0: Project Inception, Repository Setup & AI Agent Guardrails
Repository conventions, `.githooks/pre-commit` (lint, typecheck, strict tests), root `AGENTS.md`, and operational guardrails (`.agents/`).

### [x] Phase 1: Core Foundation & Scaffolding
Monorepo layout (`apps/api`, `apps/dashboard`, `services/ocr`, `infra`, `openapi`), PostgreSQL schema migrations (organizations, users, plans, api_keys, ocr_requests, usage_records, audit_logs), `/health` & `/ready` probes, and local Docker Compose.

### [x] Phase 2: API Contract, Authentication & Security Core
OpenAPI 3.1 specification (`openapi.yaml`), Swagger/Scalar UI, standardized error envelope (`request_id`), SHA-256 API key hashing lifecycle, and scoped middleware (`ocr:read`, `ocr:write`, `usage:read`).

### [x] Phase 3: OCR Engine Abstraction & Processing Pipeline
Pluggable `OCREngine` interface, synthetic `MockOCREngine`, Google Gemini Flash vision engine adapter, multipart image validation, strict 16-digit NIK validation, field confidence scoring, and zero-PII data minimization.

### [x] Phase 4: API Platform – Rate Limiting, Quotas & Usage Metering
O(1) in-memory token bucket rate limiter with standard HTTP headers (`X-RateLimit-*`, `Retry-After`), monthly quota tracking by organization plan, non-blocking asynchronous usage metering, analytics endpoints, and administrative audit logging.

### [x] Phase 5: Developer Dashboard & Portal
React + TypeScript + Tailwind CSS developer portal: real-time metric cards, quota progress bar, interactive KTP OCR playground, API key lifecycle manager, usage charts, request log explorer, and OpenAPI docs viewer.

### [x] Phase 6: Observability, Telemetry & Structured Logging
Full OpenTelemetry Go SDK instrumentation, HTTP and DB tracing spans, Prometheus metrics endpoint (`/metrics`), Grafana dashboard provisioning (RED metrics: Rate, Errors, Duration), and structured JSON logging (`slog`) with automatic PII sanitization.

### [x] Phase 7: Infrastructure as Code & Multi-Environment Provisioning
Multi-stage Dockerfiles for Go API and React SPA, OpenTofu modules (`infra/modules/`) for Azure Container Apps, PostgreSQL Flexible Server, Azure Key Vault, Cloudflare WAF/TLS, and Terragrunt staging/production configurations.

### [x] Phase 8: CI/CD Delivery Pipeline & Automated Security Gates
GitHub Actions workflows with automated quality gates: Go race detector, golangci-lint, Biome/TypeScript check, Spectral OpenAPI linter, Gitleaks, govulncheck, gosec SAST, tfsec IaC scan, Trivy container image scan, and sub-60-second automated rollback scripts.

### [x] Phase 9: Reliability Engineering, Failure Drills & E2E Testing
Headless Playwright E2E test suite covering the entire user journey, and 4 documented production failure drill post-mortems: OCR provider timeout, database outage, broken smoke test gate, and production error regression.

### [x] Phase 10: Documentation, ADRs, Demo Consumers & Portfolio Polish
Five core ADRs (`ADR-001` to `ADR-005`), production runbooks (`architecture.md`, `security.md`, `observability.md`, `rollback.md`), external client demos (`VeriForm` and `RentEase`), and polished root `README.md`.

---

## Section II: Phase 11 — Production API Platform Maturity & Enterprise Alignment (Active V2)

### Phase 11.1: Real Load Testing & RED Metrics Analysis (Priority: VERY HIGH)
*Goal: Provide empirical proof of system throughput, latency percentiles, and bottleneck limits.*

- [x] Create `tests/load/k6-baseline.js`:
  - 100 concurrent Virtual Users (VUs) over 2 minutes.
  - Test targets: `/health`, `/api/v1/auth/verify`, `/api/v1/usage`, synthetic OCR payload.
- [x] Create `tests/load/k6-stress.js`:
  - Ramp-up load to 500 rps and 1,000 rps.
  - Stress test rate limiter, token refill contention, and DB connection pool.
- [x] Run benchmarks against local instance and capture raw metrics:
  - Baseline: P50, P90, P95, P99, error rate %.
  - Stress: Saturation point, 429 rate limit distribution, DB pool wait time.
- [x] Author `docs/benchmarks/load-test-report.md`:
  - Graph/table of RED metrics under varying load.
  - Identified bottleneck & code explanation (e.g. sync.Mutex lock contention vs DB pool sizing).
- **Interview Narrative**:
  > *"Rather than claiming the system scales, I subjected the API to 1,000 rps via k6, measured our RED metrics in Prometheus, identified the exact mutex contention in our in-memory token bucket, and tuned the connection pool accordingly."*

---

### Phase 11.2: API Idempotency Subsystem (Priority: MEDIUM-HIGH)
*Goal: Guarantee safe client retries over unreliable networks without duplicate processing or duplicate quota deduction.*

- [x] Implement `apps/api/internal/idempotency/`:
  - In-memory / DB idempotency store with TTL (24h).
  - SHA-256 payload hash verification (prevent key reuse with differing payloads).
- [x] Implement middleware `apps/api/internal/http/middleware/idempotency.go`:
  - Intercept `Idempotency-Key` header.
  - Return HTTP 409 `request_in_progress` if identical request is currently processing.
  - Return cached status, headers, and body with `Idempotent-Replayed: true` if completed.
- [x] Attach idempotency middleware to `POST /api/v1/ocr/ktp` in `router.go`.
- [x] Unit & Integration tests:
  - Test replay returns identical result.
  - Test altered body with same key returns 422 Unprocessable Entity.
  - Test concurrent requests return 409 Conflict.
- **Interview Narrative**:
  > *"External API clients experience network drops. With our Idempotency-Key implementation, clients can safely retry failed network calls without paying twice or reprocessing the OCR document."*

---

### Phase 11.3: Verifiable Deployment & Rollback Evidence (Priority: VERY HIGH)
*Goal: Turn documented CI/CD promises into concrete, undeniable operational proof.*

- [x] Execute clean end-to-end CI pipeline run:
  - Capture linters, `go test -race`, `govulncheck`, `gosec`, `spectral`, and `biome` logs.
- [x] Execute simulated broken staging smoke test:
  - Deploy revision with broken health probe.
  - Capture smoke test failure output and proof of deployment gate halting promotion.
- [x] Execute production traffic shift and instant rollback:
  - Deploy v1.4.1 -> trigger regression -> rollback to v1.4.0 in < 60s.
  - Capture CLI and container logs.
- [x] Create `docs/deployment/evidence-log.md` with terminal logs, commit SHAs, and timestamped traces.
- **Interview Narrative**:
  > *"I don't just say we have automated rollback; here is the deployment audit trace where an unhealthy revision failed the post-deploy smoke test, halted the promotion gate, and safely kept traffic on the previous stable revision."*

---

### Phase 11.4: Keycloak + OIDC Human Identity (Priority: HIGH)
*Goal: Demonstrate proper dual authentication — OIDC for human operators, API keys for machines.*

- [x] Add Keycloak container to `docker-compose.yml` (`quay.io/keycloak/keycloak:24.0`).
- [x] Configure `infra/keycloak/realm-lensio.json`:
  - Realm: `lensio`.
  - Clients: `lensio-api` (bearer-only resource server), `lensio-dashboard` (public SPA client).
  - Seed users: `admin@lensio.dev`, `developer@veriform.com`.
- [x] Implement `apps/api/internal/http/middleware/oidc.go`:
  - Validate JWT bearer tokens against Keycloak JWKS endpoint.
  - Extract identity claims (`sub`, `email`, `preferred_username`).
  - Unit tests with mock JWKS/JWT generator.
- [x] Update Dashboard `AuthContext.tsx` to support both OIDC session login and API key switching.
- **Interview Narrative**:
  > *"We strictly separate human identity from machine identity. Humans access the dashboard via Keycloak OIDC with short-lived JWTs, while external partner applications authenticate using scoped, hashed API keys."*

---

### Phase 11.5: SpiceDB Fine-Grained Authorization / ReBAC (Priority: HIGH)
*Goal: Implement Zanzibar-style fine-grained relationship-based access control.*

- [x] Author Zanzibar schema `infra/spicedb/schema.zed`:
  - Definitions: `user`, `organization`, `project`, `api_key`.
  - Permissions: `project->manage_api_keys`, `api_key->revoke`, `api_key->use`.
- [x] Add SpiceDB container to `docker-compose.yml` (`authzed/spicedb`).
- [x] Implement Go client adapter in `apps/api/internal/authz/spicedb.go`:
  - `CheckPermission(ctx, resource, permission, subject) (bool, error)`.
  - `WriteRelationship(ctx, relation) error`.
  - `MockAuthorizer` for zero-dependency unit tests.
- [x] Enforce SpiceDB authorization in API key handlers:
  - Verify actor has `manage_api_keys` before creating or revoking keys.
- **Interview Narrative**:
  > *"Authentication answers 'WHO are you?' via Keycloak. Authorization answers 'WHAT are you allowed to do?' via SpiceDB. We use SpiceDB's ReBAC model to enforce organizational boundaries and project-level API key delegation."*

---

### Phase 11.6: Deep Incident Engineering & Circuit Breaker (Priority: HIGH)
*Goal: Show senior-level incident mitigation, observability correlation, and code-level resilience.*

- [x] Implement Circuit Breaker in `services/ocr/circuit_breaker.go`:
  - Three states: Closed, Open, Half-Open.
  - Configurable failure threshold, cooldown timer, and adaptive context timeout.
  - Prometheus metrics: `lensio_ocr_circuit_breaker_state`, `lensio_ocr_circuit_breaker_tripped_total`.
- [x] Integrate Circuit Breaker into `services/ocr/engine.go`.
- [x] Write post-mortem document: `docs/incidents/INC-20260912-05-ocr-provider-latency-cascade.md`:
  - Detection via Prometheus alert on P95 latency.
  - Trace span breakdown isolating Google Gemini external HTTP roundtrip.
  - Code fix, circuit breaker tripping demonstration, and graceful 504 degradation without worker pool exhaustion.
- **Interview Narrative**:
  > *"When an upstream vision API degraded, our P95 latency tripled and threatened worker thread exhaustion. We detected it via OTel span analysis, implemented an adaptive circuit breaker in Go, and verified that subsequent failures tripped the breaker in 100ms instead of hanging for 10s."*

---

### Phase 11.7: Actionable Alerting & SLO Definitions (Priority: MEDIUM)
*Goal: Define operational contracts and trigger automated notifications before customers report downtime.*

- [x] Author `infra/observability/prometheus/alerting_rules.yml`:
  - `HighErrorRate`: 5xx errors > 1% over 2 minutes.
  - `P95LatencyBreached`: P95 latency > 500ms over 5 minutes.
  - `CircuitBreakerOpen`: OCR circuit breaker entered Open state.
  - `RateLimitSurge`: 429 responses > 25% of incoming traffic.
- [x] Author `docs/observability/slo-definition.md`:
  - 99.9% Availability SLA/SLO contract.
  - Latency and Error Budget consumption calculation.
- **Interview Narrative**:
  > *"Dashboards show what is happening; alerts tell us when we must act. We established an explicit 99.9% availability SLO and configured Prometheus alert rules targeting our error budget consumption rate."*

---

### Phase 11.8: Architectural Decision Record: Rate Limiting Trade-offs (Priority: MED-HIGH)
*Goal: Articulate deep distributed systems trade-offs without over-engineering.*

- [ ] Author `docs/decisions/ADR-006-multi-instance-rate-limiting-tradeoffs.md`:
  - Evaluation of current O(1) in-memory token bucket.
  - Impact of multi-replica deployment on Azure Container Apps (drift across $N$ instances).
  - Comparative analysis: In-memory vs Distributed Redis sliding window vs Cloudflare edge rate limiting.
  - Design of future distributed Redis adapter interface.
- **Interview Narrative**:
  > *"We chose an in-memory token bucket for O(1) single-instance speed and simplicity. In our ADR-006, I documented the exact trade-offs when scaling across multiple Container App replicas: accepting coarse-grained drift or introducing a centralized Redis sliding window."*

---

### Phase 11.9: API Versioning Strategy & Contract Evolution (Priority: MEDIUM)
*Goal: Demonstrate client empathy and product engineering hygiene.*

- [ ] Author `docs/decisions/ADR-007-api-versioning-and-contract-evolution.md`:
  - Clear rules for breaking vs non-breaking changes.
  - Contract lifecycle: Deprecation headers (`Deprecation: true`, `Sunset: <date>`).
  - Safe evolution of request/response payloads.
- **Interview Narrative**:
  > *"An API is a published contract with third-party code. We never break client integrations on internal changes; contract evolutions follow our documented deprecation and sunset policy."*

---

## Section III: Future Product Roadmap (Post-V2 Expansion)

> [!NOTE]
> **Product Vision**: After the production API platform foundation is fully hardened and verified, Lensio expands horizontally into additional identity documents and automated trust services.

```text
Lensio Product Ecosystem
│
├── Document OCR Expansion
│   ├── [x] KTP OCR (Kartu Tanda Penduduk - Active Production Core)
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

---

## Definition of Done for V2 Release

- [ ] `k6` baseline and stress test scripts exist, executed, and benchmark report is committed.
- [x] `Idempotency-Key` header is supported on `POST /api/v1/ocr/ktp` with 100% test coverage.
- [x] Deployment evidence log captures real CI test passes, security scans, and smoke test rollback.
- [x] Keycloak OIDC JWT validation middleware works with unit tests and Docker Compose.
- [x] SpiceDB ReBAC authorization model is defined and protects sensitive API key mutations.
- [x] Circuit breaker protects the OCR engine from upstream latency spikes and has an incident post-mortem.
- [ ] Prometheus alerting rules and SLO definition document are committed.
- [ ] ADR-006 (Rate Limiting Trade-offs) and ADR-007 (API Versioning) are committed.
- [ ] `go test -race ./...` and dashboard tests pass cleanly with zero failures.
