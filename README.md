# NusaID

<p align="center">
  <img src="https://img.shields.io/badge/build-passing-brightgreen?style=flat-square" alt="Build Status" />
  <img src="https://img.shields.io/badge/go%20report-A%2B-brightgreen?style=flat-square" alt="Go Report Card" />
  <img src="https://img.shields.io/badge/coverage-100%25%20target-brightgreen?style=flat-square" alt="Coverage" />
  <img src="https://img.shields.io/badge/security%20gates-5%2F5%20passed-brightgreen?style=flat-square" alt="Security Gates" />
  <img src="https://img.shields.io/badge/opentelemetry-active-blue?style=flat-square" alt="OpenTelemetry" />
  <img src="https://img.shields.io/badge/rollback%20sla-%3C60s-blue?style=flat-square" alt="Rollback SLA" />
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License" />
</p>

> **"NusaID is a developer-first KTP OCR API that lets applications extract structured Indonesian KTP data through a secure, rate-limited, observable, and production-ready API."**

NusaID is built to explore what it takes to operate a production API as a managed product: from cryptographic key management and scoped authorization to token-bucket rate limiting, non-blocking usage metering, distributed tracing, automated delivery, and sub-60-second revision rollbacks.

---

## The Engineering Story

The interesting part of NusaID is not merely OCR. The true engineering challenge lies in **everything around the API**:

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

NusaID demonstrates full-lifecycle engineering capabilities across the entire platform stack:

| Domain | NusaID Production Evidence | Reference |
|---|---|---|
| **Language & Runtime** | Go 1.22+ clean architecture, native `net/http.ServeMux`, sub-80ms startup | [`ADR-001`](docs/decisions/ADR-001-why-go-for-api-and-ocr-service.md) |
| **Relational Database** | PostgreSQL 16 migrations (`golang-migrate`), connection pooling with `pgx/v5` | [`ADR-002`](docs/decisions/ADR-002-database-schema-and-api-key-hashing-strategy.md) |
| **Developer Portal** | React 18 + TypeScript SPA, type-safe API SDK, Tailwind CSS | [`apps/dashboard`](apps/dashboard) |
| **Cryptographic Security** | SHA-256 one-way API key hashing, zero plaintext secrets in storage | [`docs/security.md`](docs/security.md) |
| **Traffic Shaping** | $O(1)$ in-memory token bucket rate limiting + monthly quota enforcer | [`ADR-003`](docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md) |
| **Telemetry & Observability** | OpenTelemetry Go SDK, Prometheus RED metrics, Grafana dashboards | [`docs/observability.md`](docs/observability.md) |
| **Data Privacy & Compliance** | Ephemeral memory-only image handling, zero PII logs (UU PDP No. 27/2022) | [`ADR-005`](docs/decisions/ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md) |
| **Infrastructure as Code** | OpenTofu modules & Terragrunt live environments (Staging/Production) | [`infra/`](infra/) |
| **Cloud Hosting** | Azure Container Apps with KEDA autoscaling and Envoy ingress | [`ADR-004`](docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md) |
| **Continuous Delivery** | GitHub Actions with 5 security scanners (Gitleaks, govulncheck, gosec, Trivy) | [`.github/workflows`](.github/workflows/) |
| **Automated Rollbacks** | Immutable container revisions, sub-60-second traffic shifting drill | [`docs/rollback.md`](docs/rollback.md) |
| **Failure Resilience** | Documented post-mortem drills for 4 major production outage scenarios | [`docs/incidents`](docs/incidents/) |
| **External Consumers** | Independent client applications consuming NusaID API (VeriForm, RentEase) | [`examples/`](examples/) |

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

## Core API Contract

### Extract Indonesian KTP Document

```http
POST /api/v1/ocr/ktp HTTP/1.1
Host: api.nusaid.com
Authorization: Bearer nusa_live_9f8a3c2e1b4d5e6f...
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

NusaID is consumed by independent customer applications demonstrating real-world API product usage:

### 1. VeriForm (Identity Onboarding Platform)
Consumes NusaID to automate customer KYC onboarding:
```bash
go run ./examples/veriform/main.go \
  --api-url "http://localhost:8080" \
  --api-key "$NUSAID_API_KEY" \
  --image "tests/fixtures/synthetic/valid_ktp.jpg" \
  --min-age 17
```
*Docs*: [`examples/veriform/README.md`](examples/veriform/README.md)

### 2. RentEase (Vehicle Rental Platform)
Verifies driver license and identity clearance for self-drive car reservations:
```bash
go run ./examples/rentease/main.go \
  --api-url "http://localhost:8080" \
  --api-key "$NUSAID_API_KEY" \
  --image "tests/fixtures/synthetic/valid_ktp.jpg" \
  --min-age 21
```
*Docs*: [`examples/rentease/README.md`](examples/rentease/README.md)

### Run Both Consumers in One Command:
```bash
./scripts/run-demo-consumers.sh
```

---

## Quickstart Guide

### Prerequisites
- Go 1.22+
- Docker & Docker Compose
- Git

### 1. Clone & Configure Mandatory Git Hooks
```bash
git clone https://github.com/amirfaisalz/nusaid.git
cd nusaid
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

| Section | Document | Description |
|---|---|---|
| **Architecture** | [`docs/architecture.md`](docs/architecture.md) | C4 diagrams, component layers, and request flow |
| **Security & Privacy** | [`docs/security.md`](docs/security.md) | Threat model (STRIDE/OWASP), UU PDP compliance, key hashing |
| **Deployment** | [`docs/deployment.md`](docs/deployment.md) | Multi-environment promotion path (Dev -> Staging -> Prod) |
| **Emergency Rollback** | [`docs/rollback.md`](docs/rollback.md) | Step-by-step sub-60-second revision rollback procedures |
| **Observability** | [`docs/observability.md`](docs/observability.md) | Distributed tracing, RED metrics, Prometheus, and SLO rules |
| **Video Walkthrough** | [`docs/demo-walkthrough.md`](docs/demo-walkthrough.md) | 3-5 minute demo recording script and narration cues |
| **OpenAPI Contract** | [`openapi/openapi.yaml`](openapi/openapi.yaml) | Full OpenAPI 3.0 specification contract (`/docs`) |

### Architecture Decision Records (`docs/decisions/`)
- [`ADR-001: Why Go for API and OCR Service`](docs/decisions/ADR-001-why-go-for-api-and-ocr-service.md)
- [`ADR-002: Database Schema & API Key Hashing Strategy`](docs/decisions/ADR-002-database-schema-and-api-key-hashing-strategy.md)
- [`ADR-003: Rate Limiting & Quota Architecture`](docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md)
- [`ADR-004: Azure Container Apps vs. Kubernetes (Deliberate Simplicity)`](docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md)
- [`ADR-005: Data Minimization & PII Protection in OCR Pipelines`](docs/decisions/ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md)

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
Copyright © 2026 NusaID Contributors.
