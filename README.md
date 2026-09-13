# Lensio

<p align="center">
  <img src="https://img.shields.io/badge/build-passing-brightgreen?style=flat-square" alt="Build Status" />
  <img src="https://img.shields.io/badge/go%20report-A%2B-brightgreen?style=flat-square" alt="Go Report Card" />
  <img src="https://img.shields.io/badge/core%20backend%20coverage-93.5%25-brightgreen?style=flat-square" alt="Core Backend Coverage" />
  <img src="https://img.shields.io/badge/repo%20coverage-88.1%25-brightgreen?style=flat-square" alt="Whole Repo Coverage" />
  <img src="https://img.shields.io/badge/security%20gates-5%2F5%20passed-brightgreen?style=flat-square" alt="Security Gates" />
  <img src="https://img.shields.io/badge/opentelemetry-active-blue?style=flat-square" alt="OpenTelemetry" />
  <img src="https://img.shields.io/badge/plumbing%20benchmark-1%2C000%20RPS%20(mock)-brightgreen?style=flat-square" alt="Plumbing Benchmark 1,000 RPS (mock)" />
  <img src="https://img.shields.io/badge/mock%20p95%20latency-1.58ms-brightgreen?style=flat-square" alt="Mock P95 Latency 1.58ms" />
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License" />
</p>

> **"Lensio is a developer-first Indonesian Identity Document OCR API (KTP & SIM) — secure, rate-limited, observable, and production-ready."**

The interesting part is not OCR, but **everything around the API**: cryptographic key management, token-bucket rate limiting, non-blocking metering, distributed tracing, automated delivery, and sub-60-second rollbacks.

```text
Authentication              Reliability              Security
• Scoped API Keys           • Token Bucket Limits    • Ephemeral Buffers (No PII)
• SHA-256 Hashing           • Monthly Quotas         • 5 Automated Security Gates
• Instant Revocation        • Async Metering         • Append-Only Audit Trail
```

---

## Production Engineering Evidence Matrix (`PRD Section 38`)

| Domain                        | Lensio Production Evidence                                                    | Reference                                                                                    |
| ----------------------------- | ----------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| **Language & Runtime**        | Go 1.22+ clean architecture, native `net/http.ServeMux`, sub-80ms startup     | [`ADR-001`](docs/decisions/ADR-001-why-go-for-api-and-ocr-service.md)                        |
| **Relational Database**       | PostgreSQL 16 migrations (`golang-migrate`), `database/sql` + `pgx/v5/stdlib` | [`ADR-002`](docs/decisions/ADR-002-database-schema-and-api-key-hashing-strategy.md)          |
| **Developer Portal**          | React 19 + TypeScript SPA, type-safe API SDK, Tailwind CSS                    | [`apps/dashboard`](apps/dashboard)                                                           |
| **Cryptographic Security**    | SHA-256 one-way API key hashing, zero plaintext secrets in storage            | [`docs/security.md`](docs/security.md)                                                       |
| **Traffic Shaping**           | $O(1)$ in-memory token bucket + monthly quota enforcer                        | [`ADR-003`](docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md), [`ADR-006`](docs/decisions/ADR-006-multi-instance-rate-limiting-tradeoffs.md) |
| **Telemetry & Observability** | OpenTelemetry Go SDK, Prometheus RED metrics, Grafana dashboards              | [`docs/observability.md`](docs/observability.md)                                             |
| **Data Privacy & Compliance** | Ephemeral memory-only image handling, zero PII logs (UU PDP No. 27/2022)      | [`ADR-005`](docs/decisions/ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md) |
| **Infrastructure as Code**    | OpenTofu modules & Terragrunt live environments (Staging/Production)          | [`infra/`](infra/)                                                                           |
| **Cloud Hosting**             | Azure Container Apps with KEDA autoscaling and Envoy ingress                  | [`ADR-004`](docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md)                    |
| **Continuous Delivery**       | GitHub Actions with 5 security scanners (Gitleaks, govulncheck, gosec, Trivy) | [`.github/workflows`](.github/workflows/)                                                    |
| **Automated Rollbacks**       | Immutable container revisions, sub-60-second traffic shifting drill           | [`docs/rollback.md`](docs/rollback.md)                                                       |
| **Failure Resilience**        | Documented post-mortem drills for 5 major production outage scenarios         | [`docs/incidents`](docs/incidents/)                                                          |
| **Load Testing & Capacity**   | Empirical k6: 1,000 RPS sustained, $P_{95} = 1.58\text{ms}$, 0 errors         | [`docs/benchmarks/load-test-report.md`](docs/benchmarks/load-test-report.md)                  |
| **External Consumers**        | Independent client apps consuming Lensio API (VeriForm, RentEase)             | [`examples/`](examples/)                                                                     |

---

## System Topology

```mermaid
graph TD
    Client[API Consumers / VeriForm / RentEase] -->|HTTPS / Bearer Key| CF[Cloudflare Edge WAF & SSL]
    CF -->|Forward Ingress| ACA[Azure Container Apps]
    subgraph ACA Container Environment
        API[Go 1.22 REST API Service]
        DASH[React Developer Dashboard]
    end
    API -->|Dual Auth & Rate Limit| MemRL[In-Memory Token Bucket]
    API -->|OIDC JWT Validation| KC[Keycloak Identity Provider]
    API -->|ReBAC Permission Check| SDB[SpiceDB Authzed ReBAC]
    API -->|Scoped Lookup| PG[(PostgreSQL 16 Flexible Server)]
    API -->|Ephemeral Buffer| OCR[Pluggable OCREngine]
    OCR -.->|Production| Gemini[Google Gemini 2.0 Flash Vision]
    OCR -.->|Test Fixture| Mock[Deterministic MockOCREngine]
    API -->|Non-Blocking Meter| Chan[Buffered Channel Worker]
    Chan -->|Async Insert| PG
    API -->|Traces & Metrics| OTel[OpenTelemetry / Prometheus / Grafana]
```

## Technology Stack

| Layer | Components & Roles |
|---|---|
| **Backend** | Go 1.22+ (`apps/api`), native `net/http.ServeMux`, SHA-256 API keys, $O(1)$ token bucket, buffered-channel metering; PostgreSQL 16 via `pgx/v5/stdlib` + `golang-migrate`; Keycloak OIDC (human SSO) + SpiceDB ReBAC (tenant isolation) |
| **Vision AI** | Google Gemini 2.0 Flash Vision behind pluggable `OCREngine`; deterministic `MockOCREngine` for offline/CI testing |
| **Portal** | React 19 + TypeScript SPA (`apps/dashboard`), Vite + Tailwind, Vitest + Testing Library |
| **Telemetry** | OpenTelemetry Go SDK (W3C traces, `trace_id`/`request_id` correlation), Prometheus RED metrics, Grafana dashboards |
| **Cloud & Edge** | Cloudflare (DNS, DDoS, TLS 1.3, WAF) → Azure Container Apps (KEDA, Envoy, revision rollback); Key Vault secrets; OpenTofu + Terragrunt IaC |
| **Security & CI** | Distroless images; Gitleaks, govulncheck, gosec, Trivy; 4-layer pre-commit gate (build → lint → `go test -race` → PII/Big-O audit) |

---

## End-to-End Flow

`POST /api/v1/ocr/ktp` (Bearer `lensio_live_xxx`, multipart `document`, max 5MB) flows through 4 phases — full sequence diagram and walkthrough live in [`docs/architecture.md`](docs/architecture.md):

1. **Auth & guardrails** — `request_id` + OTel span → SHA-256 lookup + `ocr:write` scope → per-identity token bucket ($O(1)$) → monthly quota check.
2. **In-memory OCR** — image stays in RAM (`[]byte`, never disk) → magic-byte validation → `OCREngine` (Gemini/Mock) → deterministic NIK/SIM validation.
3. **Response** — `200 OK` JSON (<2,000ms SLA), buffer released immediately (UU PDP compliance).
4. **Async telemetry** — usage event → buffered channel → background PG insert; Prometheus metrics + trace close.

Key lifecycle: dashboard generates key → SHA-256 stored (masked prefix only) → plaintext shown **once**, never persisted. Probes: `GET /health` (liveness) vs `GET /ready` (PG pool, 503 stops routing without crash-loop).

---

## Core API Contract

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

#### Successful Response (`200 OK`) — SYNTHETIC EXAMPLE DATA

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
  "processing": { "latency_ms": 1420 }
}
```

<details>
<summary><strong>SIM endpoint + error envelope</strong></summary>

```http
POST /api/v1/ocr/sim HTTP/1.1
Host: api.lensio.dev
Authorization: Bearer lensio_live_9f8a3c2e1b4d5e6f...
Content-Type: multipart/form-data; boundary=----WebKitFormBoundary
```

```json
{
  "id": "ocr_sim_01JABC1234567890",
  "status": "completed",
  "document_type": "sim",
  "confidence": 0.98,
  "data": {
    "nomor_sim": "123456789012",
    "nama": "BUDI SANTOSO",
    "tempat_lahir": "JAKARTA",
    "tanggal_lahir": "1992-08-17",
    "golongan_darah": "O",
    "jenis_kelamin": "PRIA",
    "alamat": "JL. MERDEKA NO. 45 RT 005 RW 002 GAMBIR JAKARTA PUSAT",
    "pekerjaan": "KARYAWAN SWASTA",
    "provinsi": "DKI JAKARTA",
    "jenis_sim": "A",
    "masa_berlaku": "2029-08-17"
  },
  "processing": { "latency_ms": 1380 }
}
```

```json
{ "error": { "code": "rate_limit_exceeded", "message": "API rate limit exceeded. Please retry after 42 seconds.", "request_id": "req_01JABC123" } }
```

Error codes: `invalid_request`, `invalid_api_key`, `insufficient_scope`, `rate_limit_exceeded`, `quota_exceeded`, `invalid_document`, `unsupported_document`, `ocr_failed`, `low_confidence`, `internal_error`.

</details>

---

## Demo Consumers & Benchmarks

| Consumer | Purpose | Run |
|---|---|---|
| **VeriForm** ([docs](examples/veriform/README.md)) | KYC onboarding | `go run ./examples/veriform/main.go --api-url http://localhost:8080 --api-key "$LENSIO_API_KEY" --image tests/fixtures/synthetic/valid_ktp.jpg --min-age 17` |
| **RentEase** ([docs](examples/rentease/README.md)) | Driver-license clearance | `go run ./examples/rentease/main.go --api-url http://localhost:8080 --api-key "$LENSIO_API_KEY" --image tests/fixtures/synthetic/valid_ktp.jpg --min-age 21` |

Both at once: `./scripts/run-demo-consumers.sh`

**k6 load test** (platform plumbing via `MockOCREngine`; live Gemini adds 1,200–2,000ms upstream, isolated by circuit breaker):

| Metric | 100 VUs baseline | 1,000 RPS stress |
|---|---|---|
| Throughput | 852.84 req/s | 1,000.00 req/s sustained |
| 5xx errors | 0 | 0 |
| P50 / P95 | 261.67 µs / **1.58 ms** | 304.56 µs / **1.21 ms** |
| 429 throttled | 91,140 deterministic | 50,649 deterministic |
| DB pool `wait_count` | 0 | 0 |

Takeaways: async metering shielded Postgres; same-tenant bursts serialize on the limiter mutex (~32ms tail → Redis roadmap, ADR-006). Full report: [`docs/benchmarks/load-test-report.md`](docs/benchmarks/load-test-report.md) · Suites: [`tests/load/`](tests/load/) · Runner: `./scripts/run-load-tests.sh`

---

## Quickstart

Prerequisites: **Go 1.26** (minimum 1.22), Docker & Compose, Git.

> Coverage note: badges reflect the last measurement (core ~93.5%, repo ~88%). Target 100% belum tercapai — lihat `coverage.out` lokal setelah `go test -race -cover ./...`.
> Semua contoh identitas di README ini adalah **SYNTHETIC**. Lihat [`PRIVACY.md`](PRIVACY.md) dan [`USE_POLICY.md`](USE_POLICY.md) sebelum memproses KTP asli.

```bash
git clone https://github.com/amirfaisalz/lensio.git
cd lensio
git config core.hooksPath .githooks
docker compose up -d
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
go test -race -cover ./...
```

---

## Documentation Index

| Section | Document |
|---|---|
| Architecture | [`docs/architecture.md`](docs/architecture.md) |
| Security & Privacy | [`docs/security.md`](docs/security.md) |
| Deployment | [`docs/deployment.md`](docs/deployment.md) |
| Emergency Rollback | [`docs/rollback.md`](docs/rollback.md) |
| Observability | [`docs/observability.md`](docs/observability.md) |
| Load Testing Report | [`docs/benchmarks/load-test-report.md`](docs/benchmarks/load-test-report.md) |
| Video Walkthrough | [`docs/demo-walkthrough.md`](docs/demo-walkthrough.md) |
| OpenAPI Contract | [`openapi/openapi.yaml`](openapi/openapi.yaml) |
| Privacy (UU PDP) | [`PRIVACY.md`](PRIVACY.md) |
| Security Policy | [`SECURITY.md`](SECURITY.md) |
| Acceptable Use | [`USE_POLICY.md`](USE_POLICY.md) |
| Contributing | [`CONTRIBUTING.md`](CONTRIBUTING.md) |
| Code of Conduct | [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) |
| Changelog | [`CHANGELOG.md`](CHANGELOG.md) |

ADRs: [`ADR-001`](docs/decisions/ADR-001-why-go-for-api-and-ocr-service.md) · [`ADR-002`](docs/decisions/ADR-002-database-schema-and-api-key-hashing-strategy.md) · [`ADR-003`](docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md) · [`ADR-004`](docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md) · [`ADR-005`](docs/decisions/ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md) · [`ADR-006`](docs/decisions/ADR-006-multi-instance-rate-limiting-tradeoffs.md) · [`ADR-007`](docs/decisions/ADR-007-api-versioning-and-contract-evolution.md)

Incidents: [`INC-01`](docs/incidents/INC-20260912-01-ocr-provider-timeout.md) · [`INC-02`](docs/incidents/INC-20260912-02-database-outage.md) · [`INC-03`](docs/incidents/INC-20260912-03-broken-deployment-smoke-test.md) · [`INC-04`](docs/incidents/INC-20260912-04-production-regression-drill.md) · [`INC-05`](docs/incidents/INC-20260912-05-ocr-provider-latency-cascade.md)

---

## Pre-Commit Verification Gate

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
