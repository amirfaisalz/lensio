# Lensio

<p align="center">
  <img src="https://img.shields.io/badge/build-passing-brightgreen?style=flat-square" alt="Build Status" />
  <img src="https://img.shields.io/badge/go%20report-A%2B-brightgreen?style=flat-square" alt="Go Report Card" />
  <img src="https://img.shields.io/badge/coverage-100%25%20target-brightgreen?style=flat-square" alt="Coverage" />
  <img src="https://img.shields.io/badge/security%20gates-5%2F5%20passed-brightgreen?style=flat-square" alt="Security Gates" />
  <img src="https://img.shields.io/badge/opentelemetry-active-blue?style=flat-square" alt="OpenTelemetry" />
  <img src="https://img.shields.io/badge/load%20test-1%2C000%20RPS%20sustained-brightgreen?style=flat-square" alt="Load Test 1,000 RPS" />
  <img src="https://img.shields.io/badge/p95%20latency-1.58ms-brightgreen?style=flat-square" alt="P95 Latency 1.58ms" />
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License" />
</p>

> **"Lensio is a developer-first KTP OCR API that lets applications extract structured Indonesian KTP data through a secure, rate-limited, observable, and production-ready API."**

Lensio is built to explore what it takes to operate a production API as a managed product: from cryptographic key management and scoped authorization to token-bucket rate limiting, non-blocking usage metering, distributed tracing, automated delivery, and sub-60-second revision rollbacks.

---

## The Engineering Story

The interesting part of Lensio is not merely OCR. The true engineering challenge lies in **everything around the API**:

```text
                ┌────────────────────────┐
                │   KTP OCR API Product  │
                └───────────┬────────────┘
                            │
       ┌────────────────────┼────────────────────┐
       │                    │                    │
 Authentication        Reliability            Security
       │                    │                    │
 • Scoped API Keys     • Token Bucket Limits • Ephemeral Buffers (No PII)
 • SHA-256 Hashing     • Monthly Quotas      • 5 Automated Security Gates
 • Instant Revocation  • Async Metering      • Append-Only Audit Trail
       │                    │                    │
       └────────────────────┼────────────────────┘
                            │
                     Production Path
                            │
               CI (Gates & Race Detector)
                            ↓
                    Azure Container Apps
                            ↓
               Instant Rollback (<60s SLA)
```

---

## Production Engineering Evidence Matrix (`PRD Section 38`)

Lensio demonstrates full-lifecycle engineering capabilities across the entire platform stack:

| Domain                        | Lensio Production Evidence                                                    | Reference                                                                                    |
| ----------------------------- | ----------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| **Language & Runtime**        | Go 1.22+ clean architecture, native `net/http.ServeMux`, sub-80ms startup     | [`ADR-001`](docs/decisions/ADR-001-why-go-for-api-and-ocr-service.md)                        |
| **Relational Database**       | PostgreSQL 16 migrations (`golang-migrate`), connection pooling with `pgx/v5` | [`ADR-002`](docs/decisions/ADR-002-database-schema-and-api-key-hashing-strategy.md)          |
| **Developer Portal**          | React 19 + TypeScript SPA, type-safe API SDK, Tailwind CSS                    | [`apps/dashboard`](apps/dashboard)                                                           |
| **Cryptographic Security**    | SHA-256 one-way API key hashing, zero plaintext secrets in storage            | [`docs/security.md`](docs/security.md)                                                       |
| **Traffic Shaping**           | $O(1)$ in-memory token bucket rate limiting + monthly quota enforcer          | [`ADR-003`](docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md), [`ADR-006`](docs/decisions/ADR-006-multi-instance-rate-limiting-tradeoffs.md) |
| **Telemetry & Observability** | OpenTelemetry Go SDK, Prometheus RED metrics, Grafana dashboards              | [`docs/observability.md`](docs/observability.md)                                             |
| **Data Privacy & Compliance** | Ephemeral memory-only image handling, zero PII logs (UU PDP No. 27/2022)      | [`ADR-005`](docs/decisions/ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md) |
| **Infrastructure as Code**    | OpenTofu modules & Terragrunt live environments (Staging/Production)          | [`infra/`](infra/)                                                                           |
| **Cloud Hosting**             | Azure Container Apps with KEDA autoscaling and Envoy ingress                  | [`ADR-004`](docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md)                    |
| **Continuous Delivery**       | GitHub Actions with 5 security scanners (Gitleaks, govulncheck, gosec, Trivy) | [`.github/workflows`](.github/workflows/)                                                    |
| **Automated Rollbacks**       | Immutable container revisions, sub-60-second traffic shifting drill           | [`docs/rollback.md`](docs/rollback.md)                                                       |
| **Failure Resilience**        | Documented post-mortem drills for 4 major production outage scenarios         | [`docs/incidents`](docs/incidents/)                                                          |
| **Load Testing & Capacity**   | Empirical k6 testing: 1,000 RPS sustained, 225k+ reqs, $P_{95} = 1.58\text{ms}$, 0 errors | [`docs/benchmarks/load-test-report.md`](docs/benchmarks/load-test-report.md)                  |
| **External Consumers**        | Independent client applications consuming Lensio API (VeriForm, RentEase)     | [`examples/`](examples/)                                                                     |

---

## System Topology & Architecture

```mermaid
graph TD
    Client[API Consumers / VeriForm / RentEase] -->|HTTPS / Bearer Key| CF[Cloudflare Edge WAF & SSL]
    CF -->|Forward Ingress| ACA[Azure Container Apps]

    subgraph ACA Container Environment
        API[Go 1.22 REST API Service]
        DASH[React Developer Dashboard]
    end

    API -->|1. Auth & Rate Limit| MemRL[In-Memory Token Bucket]
    API -->|2. Scoped Lookup| PG[(PostgreSQL 16 Flexible Server)]
    API -->|3. Ephemeral Buffer| OCR[Pluggable OCREngine]
    OCR -.->|Production| Gemini[Google Gemini 2.0 Flash Vision]
    OCR -.->|Test Fixture| Mock[Deterministic MockOCREngine]
    API -->|4. Non-Blocking Meter| Chan[Buffered Channel Worker]
    Chan -->|Async Insert| PG
    API -->|5. Traces & Metrics| OTel[OpenTelemetry / Prometheus / Grafana]
```

---

## Technology Stack & Component Roles

Lensio combines a high-performance Go backend, a reactive TypeScript developer portal, cloud-native container hosting, and an end-to-end observability stack. Below is what each technology does in the project:

### 1. Backend Core & API Runtime

- **Go (Golang 1.22+)**:
  - Powers the core REST API (`apps/api`).
  - Built with clean, hexagonal architecture using native `net/http.ServeMux` (zero heavy web frameworks) for minimal binary size and sub-80ms startup.
  - Implements concurrent request handling, SHA-256 cryptographic API key hashing, $O(1)$ token-bucket rate limiting, and buffered Go channels for non-blocking asynchronous metering.
- **PostgreSQL 16**:
  - Primary relational datastore storing organizations, hashed API keys (`api_keys`), monthly quota plans (`plans`), append-only usage logs (`usage_records`), and administrative audit logs (`audit_logs`).
  - Schema migrations are version-controlled and executed using `golang-migrate`.
- **`jackc/pgx/v5`**:
  - High-performance native PostgreSQL driver and connection pool (`pgxpool`) providing low-latency database queries with connection lifecycle management and health checks (`/ready` probe).

### 2. Vision AI & Document Extraction

- **Google Gemini 2.0 Flash Vision**:
  - Primary multimodal AI vision engine that processes KTP document images and extracts structured Indonesian identity fields (NIK, Full Name, Birth Date/Place, Gender, Address, Religion, Marital Status).
- **Pluggable `OCREngine` Interface**:
  - Decouples HTTP handlers from external AI providers. Enables hot-swapping or fallback across different AI models without modifying business logic.
- **Deterministic `MockOCREngine`**:
  - Built-in synthetic fixture engine providing offline, reproducible, and sub-millisecond OCR responses during unit, integration, and CI/CD testing.

### 3. Developer Portal & User Interface

- **React 19 & TypeScript (`apps/dashboard`)**:
  - Single Page Application (SPA) developer portal allowing users to create scoped API keys, view masked credentials, revoke compromised keys, monitor daily request volume, inspect recent request logs, and read interactive API documentation.
- **Vite & Tailwind CSS**:
  - Ultra-fast bundling, instant HMR, and utility-first styling for a responsive, modern dark-mode developer experience.
- **Vitest & React Testing Library**:
  - Test framework providing 100% component test coverage and client API contract verification.

### 4. Telemetry & Observability Stack

- **OpenTelemetry (OTel) Go SDK**:
  - Injects W3C distributed trace contexts, tracks spans across middleware, database queries, and OCR calls, and correlates traces with structured logs via `trace_id` and `request_id`.
- **Prometheus**:
  - Periodically scrapes the `/metrics` endpoint to record RED metrics (Rate, Errors, Duration) and business KPIs (total extractions, confidence distributions).
- **Grafana**:
  - Provides pre-configured, production-ready operational dashboards:
    - `lensio-red-metrics.json`: Latency percentiles (P50, P95, P99), request throughput, error rates.
    - `lensio-slo-business.json`: Success ratios, quota utilization, and OCR confidence tracking.

### 5. Cloud Platform, Edge & Infrastructure as Code

- **Cloudflare**:
  - Global edge proxy handling DNS, DDoS mitigation, SSL/TLS 1.3 termination, and Web Application Firewall (WAF) filtering before traffic reaches backend containers.
- **Azure Container Apps (ACA)**:
  - Serverless container execution platform running the Go API and Dashboard containers with Envoy ingress, KEDA event-driven autoscaling, and revision-based blue/green traffic shifting (<60s instant rollback).
- **Azure Key Vault**:
  - Secure hardware-backed secret management holding database passwords, AI Studio API keys, and environment secrets injected securely into container runtime environments.
- **OpenTofu & Terragrunt**:
  - Declarative Infrastructure as Code (IaC) orchestrating modular cloud resources (`infra/modules/`) across staging and production environments (`infra/live/`) with automated `tofu test` validation.

### 6. Security Gates & CI/CD

- **Docker & Google Distroless**:
  - Multi-stage Docker builds creating lightweight, immutable containers without shell utilities or package managers, minimizing security attack vectors.
- **Automated Security Toolchain**:
  - **Gitleaks**: Scans commits and PRs to prevent secret and credential leaks.
  - **govulncheck**: Scans Go dependencies against the official Go Vulnerability Database.
  - **gosec**: Static AST analysis checking Go source code for security flaws.
  - **Trivy**: Scans container images for OS and dependency vulnerabilities.
  - **Pre-Commit Git Hook (`.githooks/pre-commit`)**: 4-layer local gate enforcing compilation, linting, race-detector tests, and PII leak audits before any commit is accepted.

---

## End-to-End System Flow

The diagram and lifecycle breakdown below describe how data moves through Lensio across three primary operational flows:

```mermaid
sequenceDiagram
    autonumber
    actor Client as API Consumer / VeriForm
    participant Edge as Cloudflare Edge (WAF & TLS)
    participant Ingress as Azure Container Apps (Envoy)
    participant MW as Go Middleware Chain
    participant Quota as Quota & Plan Evaluator
    participant Engine as OCREngine (Gemini / Mock)
    participant Val as Deterministic Validator
    participant Worker as Async Metering Channel
    participant DB as PostgreSQL 16
    participant OTel as OpenTelemetry / Prometheus

    %% Flow: OCR Request
    Client->>Edge: POST /api/v1/ocr/ktp (Bearer lensio_live_xxx, multipart/form-data)
    Edge->>Ingress: Validate SSL & Route to API Container
    Ingress->>MW: Forward HTTP Request with X-Request-ID

    rect rgb(240, 245, 255)
    Note over MW,DB: Phase 1: Authentication & Traffic Guardrails
    MW->>MW: Ingest RequestID & Start OTel Span
    MW->>MW: Compute SHA-256(token)
    MW->>DB: Query API key by hash (Index scan)
    DB-->>MW: Key valid (org_id, scopes: [ocr:write])
    MW->>MW: Check in-memory Token Bucket (O(1))
    MW->>Quota: Verify monthly quota allowance
    Quota->>DB: SELECT COUNT(*) FROM usage_records WHERE org_id=$1
    DB-->>Quota: Current usage < Monthly quota
    end

    rect rgb(245, 255, 245)
    Note over MW,Val: Phase 2: In-Memory OCR & Deterministic Validation
    MW->>Engine: Stream image []byte in RAM (No disk persistence)
    Engine-->>MW: Raw extracted identity fields
    MW->>Val: Run deterministic validation (NIK 16-digit structure, province, DOB)
    Val-->>MW: Sanitized data + Confidence score (e.g. 0.98)
    end

    rect rgb(255, 250, 240)
    Note over MW,Client: Phase 3: Response Dispatch (<2,000ms SLA)
    MW-->>Client: HTTP 200 OK (Clean JSON response payload)
    MW->>MW: Free image RAM buffer immediately (UU PDP compliance)
    end

    rect rgb(250, 240, 255)
    Note over Worker,OTel: Phase 4: Non-Blocking Telemetry & Metering (Async)
    MW-)Worker: Push UsageRecord to buffered channel (chan UsageEvent)
    Worker-)DB: Background worker goroutine executes INSERT INTO usage_records
    MW-)OTel: Emit RED metrics (latency, status=200) & close trace span
    end
```

### Detailed Flow Walkthrough

#### 1. Developer Key Management Flow

1. **Self-Service Generation**: A developer logs into the **React Developer Dashboard** and requests a new API key.
2. **Key Creation**: The Go backend generates a cryptographically random token with the prefix `lensio_live_` or `lensio_test_`.
3. **One-Way Hashing**: The API computes the **SHA-256 hash** of the key. The hash and masked prefix (`lensio_live_1234••••••••`) are saved into **PostgreSQL**.
4. **Single Reveal**: The plaintext key is returned to the developer **once** in the UI. Plaintext keys are never persisted anywhere in the system.

#### 2. KTP OCR Request Execution Flow

1. **Edge Inspection**: The API consumer (e.g., VeriForm onboarding service) sends a `POST /api/v1/ocr/ktp` request. **Cloudflare** terminates TLS 1.3, applies DDoS mitigation, and forwards the request to **Azure Container Apps**.
2. **Context & Tracing Initialization**: The Go `net/http` router attaches a unique `request_id` and starts an **OpenTelemetry** trace span.
3. **Authentication & Rate Limiting**:
   - The token is extracted from the `Authorization: Bearer <token>` header and hashed with SHA-256.
   - The database verifies the key's validity and ensures the `ocr:write` scope is granted.
   - An in-memory **Token Bucket** verifies the tenant is within their request-per-second rate limit ($O(1)$ lookup).
4. **Quota Verification**: The quota middleware queries PostgreSQL to ensure the organization has not exceeded its allocated monthly document quota.
5. **Data Minimization & In-Memory Processing**:
   - In accordance with Indonesia's Personal Data Protection law (**UU PDP No. 27/2022**), the uploaded KTP image is kept exclusively in **volatile RAM** (`[]byte`). It is never written to disk, local temp files, or cloud object storage.
   - Magic bytes are checked to ensure valid image formats (JPEG/PNG, max 5MB).
6. **Vision AI Extraction**:
   - The image buffer is dispatched via `OCREngine` to **Google Gemini 2.0 Flash Vision** (or `MockOCREngine` during automated testing).
   - The model parses the visual card layout and extracts raw textual attributes.
7. **Deterministic Validation**:
   - The output is normalized and validated against strict Indonesian administrative rules:
     - NIK length (exactly 16 digits).
     - Valid 2-digit province code (e.g., `31` for DKI Jakarta, `52` for NTB, `53` for NTT).
     - Valid date of birth encoding within digits 7–12 (including the +40 offset for females).
8. **Client Response Dispatch**: The validated JSON payload is serialized and returned to the client as an HTTP `200 OK` response within the target sub-2,000ms latency SLA. The memory buffer holding the raw image is released immediately.
9. **Non-Blocking Telemetry & Metering**:
   - To keep client response times fast, usage metering is **completely decoupled** from the HTTP lifecycle.
   - A `UsageRecord` event is pushed into a **buffered Go channel** (`chan UsageEvent`).
   - A background worker goroutine pulls events from the channel and asynchronously inserts them into the `usage_records` table in PostgreSQL.
   - Prometheus metrics (`http_requests_total`, `http_request_duration_seconds`, `ocr_extractions_total`) and distributed trace spans are exported.

#### 3. Health Monitoring & Failure Isolation Flow

- **Liveness Probe (`GET /health`)**: Verifies the internal Go HTTP listener is active. It does not depend on external services. If this probe fails, ACA restarts the deadlocked container.
- **Readiness Probe (`GET /ready`)**: Tests the PostgreSQL connection pool using `db.PingContext(ctx)`. If PostgreSQL becomes unreachable, the probe returns `503 Service Unavailable`, prompting ACA/Envoy to stop routing new requests to the instance without triggering destructive container crash-loops.

---

## Core API Contract

### Extract Indonesian KTP Document

```http
POST /api/v1/ocr/ktp HTTP/1.1
Host: api.lensio.dev
Authorization: Bearer lensio_live_9f8a3c2e1b4d5e6f...
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary

------WebKitFormBoundary
Content-Disposition: form-data; name="document"; filename="ktp.jpg"
Content-Type: image/jpeg

<binary image data>
------WebKitFormBoundary--
```

#### Successful Response (`200 OK`)

```json
{
  "id": "ocr_01JABC1234567890",
  "status": "completed",
  "document_type": "ktp",
  "confidence": 0.98,
  "data": {
    "nik": "3171012345670001",
    "nama": "BUDI SANTOSO",
    "tempat_lahir": "JAKARTA",
    "tanggal_lahir": "1992-08-17",
    "jenis_kelamin": "LAKI-LAKI",
    "alamat": "JL. MERDEKA NO. 45",
    "rt_rw": "005/002",
    "kelurahan": "GAMBIR",
    "kecamatan": "GAMBIR",
    "agama": "ISLAM",
    "status_perkawinan": "KAWIN",
    "pekerjaan": "KARYAWAN SWASTA",
    "kewarganegaraan": "WNI"
  },
  "processing": {
    "latency_ms": 1420
  }
}
```

#### Standardized Error Envelope (`PRD Section 20`)

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "API rate limit exceeded. Please retry after 42 seconds.",
    "request_id": "req_01JABC123"
  }
}
```

Standard error codes: `invalid_request`, `invalid_api_key`, `insufficient_scope`, `rate_limit_exceeded`, `quota_exceeded`, `invalid_document`, `unsupported_document`, `ocr_failed`, `low_confidence`, `internal_error`.

---

## External Demo Consumers

Lensio is consumed by independent customer applications demonstrating real-world API product usage:

### 1. VeriForm (Identity Onboarding Platform)

Consumes Lensio to automate customer KYC onboarding:

```bash
go run ./examples/veriform/main.go \
  --api-url "http://localhost:8080" \
  --api-key "$LENSIO_API_KEY" \
  --image "tests/fixtures/synthetic/valid_ktp.jpg" \
  --min-age 17
```

_Docs_: [`examples/veriform/README.md`](examples/veriform/README.md)

### 2. RentEase (Vehicle Rental Platform)

Verifies driver license and identity clearance for self-drive car reservations:

```bash
go run ./examples/rentease/main.go \
  --api-url "http://localhost:8080" \
  --api-key "$LENSIO_API_KEY" \
  --image "tests/fixtures/synthetic/valid_ktp.jpg" \
  --min-age 21
```

_Docs_: [`examples/rentease/README.md`](examples/rentease/README.md)

### Run Both Consumers in One Command:

```bash
./scripts/run-demo-consumers.sh
```

---

## Empirical Performance & RED Metrics (k6 Load Testing)

Rather than relying on theoretical claims, Lensio's production platform was subjected to rigorous empirical load testing using **Grafana k6**, instrumented with **OpenTelemetry**, and monitored through **Prometheus** RED metrics (*Rate, Errors, Duration*):

| Metric / Condition | Baseline Concurrency (100 VUs) | Saturation Stress (Ramp to 1,000 RPS) |
|---|---|---|
| **Total Invocations** | **140,803 requests** (over 2m45s) | **85,124 requests** (over 2m30s) |
| **Throughput** | **852.84 req/s** steady state | **1,000.00 req/s** peak sustained |
| **HTTP Error Rate (5xx)** | **0 (0.00%)** — Zero Crashes | **0 (0.00%)** — Zero Crashes |
| **Checks Succeeded** | **100.00%** (302,940 / 302,940) | **100.00%** (110,657 / 110,657) |
| **Rate Limit 429 Events** | 91,140 throttled deterministically | 50,649 throttled deterministically |
| **P50 Latency (Median)** | **261.67 µs** (sub-millisecond) | **304.56 µs** (sub-millisecond) |
| **P90 Latency** | **842.67 µs** | **763.61 µs** |
| **P95 Latency** | **1.58 ms** | **1.21 ms** |
| **P99 Latency** | **4.94 ms** | — |
| **DB Pool Saturation** | `wait_count = 0` (4 open, 3 idle) | `wait_count = 0` (4 open, 3 idle) |

### Key Empirical Findings & Architectural Takeaways:
1. **Non-Blocking Usage Metering Shielded PostgreSQL (`wait_count = 0`)**:  
   The decision to buffer metering logs in an asynchronous channel (`usage.Recorder`, 1024-deep ring buffer) completely prevented PostgreSQL connection pool exhaustion under 1,000 RPS, keeping connection wait counts at zero.
2. **Identified Real-World Bottleneck**:  
   Under concurrent bursts carrying API keys for the same tenant organization, goroutines serialize on `sync.Mutex` (`b.mu.Lock()` in `ratelimit.Limiter.Allow()`), inducing tail latency spikes up to ~32ms. This empirical evidence validates our architectural roadmap for a partitioned/sharded limiter and distributed Redis token bucket (Phase 11.8).
3. **Deterministic RFC-Compliant Throttling**:  
   Handled over 141,000 rate limit events with accurate `Retry-After` and `X-RateLimit-*` headers and zero memory leaks.

- **Full Benchmark Report**: [`docs/benchmarks/load-test-report.md`](docs/benchmarks/load-test-report.md)
- **Load Test Suites**: [`tests/load/`](tests/load/) | **Automated Runner**: `./scripts/run-load-tests.sh`

---

## Quickstart Guide

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Git

### 1. Clone & Configure Mandatory Git Hooks

```bash
git clone https://github.com/amirfaisalz/lensio.git
cd lensio
git config core.hooksPath .githooks
```

### 2. Start Local Environment (PostgreSQL + API)

```bash
docker compose up -d
```

### 3. Verify Health Probes

```bash
# Liveness probe (verifies process responsiveness)
curl -i http://localhost:8080/health

# Readiness probe (verifies database connection pool)
curl -i http://localhost:8080/ready

# Prometheus metrics
curl -i http://localhost:8080/metrics
```

### 4. Run Strict Test Suite with Race Detector

```bash
go test -race -cover ./...
```

---

## Documentation Directory Index

| Section                | Document                                               | Description                                                 |
| ---------------------- | ------------------------------------------------------ | ----------------------------------------------------------- |
| **Architecture**       | [`docs/architecture.md`](docs/architecture.md)         | C4 diagrams, component layers, and request flow             |
| **Security & Privacy** | [`docs/security.md`](docs/security.md)                 | Threat model (STRIDE/OWASP), UU PDP compliance, key hashing |
| **Deployment**         | [`docs/deployment.md`](docs/deployment.md)             | Multi-environment promotion path (Dev -> Staging -> Prod)   |
| **Emergency Rollback** | [`docs/rollback.md`](docs/rollback.md)                 | Step-by-step sub-60-second revision rollback procedures     |
| **Observability**      | [`docs/observability.md`](docs/observability.md)       | Distributed tracing, RED metrics, Prometheus, and SLO rules |
| **Load Testing Report** | [`docs/benchmarks/load-test-report.md`](docs/benchmarks/load-test-report.md) | Empirical k6 load test results, RED metrics, and bottlenecks |
| **Video Walkthrough**  | [`docs/demo-walkthrough.md`](docs/demo-walkthrough.md) | 3-5 minute demo recording script and narration cues         |
| **OpenAPI Contract**   | [`openapi/openapi.yaml`](openapi/openapi.yaml)         | Full OpenAPI 3.0 specification contract (`/docs`)           |

### Architecture Decision Records (`docs/decisions/`)

- [`ADR-001: Why Go for API and OCR Service`](docs/decisions/ADR-001-why-go-for-api-and-ocr-service.md)
- [`ADR-002: Database Schema & API Key Hashing Strategy`](docs/decisions/ADR-002-database-schema-and-api-key-hashing-strategy.md)
- [`ADR-003: Rate Limiting & Quota Architecture`](docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md)
- [`ADR-004: Azure Container Apps vs. Kubernetes (Deliberate Simplicity)`](docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md)
- [`ADR-005: Data Minimization & PII Protection in OCR Pipelines`](docs/decisions/ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md)
- [`ADR-006: Multi-Instance Rate Limiting Trade-offs (Distributed vs. Local)`](docs/decisions/ADR-006-multi-instance-rate-limiting-tradeoffs.md)

### Incident Reports & Failure Drills (`docs/incidents/`)

- [`INC-20260912-01: Vision AI OCR Provider Latency & Timeout Simulation`](docs/incidents/INC-20260912-01-ocr-provider-timeout.md)
- [`INC-20260912-02: PostgreSQL Outage & Readiness Probe Isolation`](docs/incidents/INC-20260912-02-database-outage.md)
- [`INC-20260912-03: Broken Deployment Smoke Test Promotion Halt`](docs/incidents/INC-20260912-03-broken-deployment-smoke-test.md)
- [`INC-20260912-04: Production Regression Drill & Traffic Shift Rollback (<60s)`](docs/incidents/INC-20260912-04-production-regression-drill.md)

---

## Pre-Commit Verification Gate

Every commit is strictly verified by `.githooks/pre-commit` across four mandatory layers:

```text
Layer 1: Type Checking & Compilation (go build ./..., tsc --noEmit)
Layer 2: Linting & Code Quality (golangci-lint run / go vet, frontend lint)
Layer 3: Strict Tests & 100% Coverage Target with Race Detector (go test -race -cover ./...)
Layer 4: Big O Sanity & Security Guardrails (PII leak audit & scratch hygiene)
```

---

## License

Distributed under the [MIT License](LICENSE).  
Copyright © 2026 Lensio Contributors.
