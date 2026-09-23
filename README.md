# Lensio

<p align="center">
  <img src="apps/dashboard/public/lensio-logo.png" alt="Lensio logo" width="120" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/build-passing-brightgreen?style=flat-square" alt="Build Status" />
  <img src="https://img.shields.io/badge/go%20report-A%2B-brightgreen?style=flat-square" alt="Go Report Card" />
  <img src="https://img.shields.io/badge/core%20backend%20coverage-88.5%25-brightgreen?style=flat-square" alt="Core Backend Coverage" />
  <img src="https://img.shields.io/badge/repo%20coverage-87.5%25-brightgreen?style=flat-square" alt="Whole Repo Coverage" />
  <img src="https://img.shields.io/badge/security%20gates-5%2F5%20passed-brightgreen?style=flat-square" alt="Security Gates" />
  <img src="https://img.shields.io/badge/opentelemetry-active-blue?style=flat-square" alt="OpenTelemetry" />
  <img src="https://img.shields.io/badge/plumbing%20benchmark-1%2C000%20RPS%20(mock)-brightgreen?style=flat-square" alt="Plumbing Benchmark 1,000 RPS (mock)" />
  <img src="https://img.shields.io/badge/mock%20p95%20latency-1.58ms-brightgreen?style=flat-square" alt="Mock P95 Latency 1.58ms" />
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License" />
</p>

> **Lensio is a developer-first Indonesian Identity Document OCR API (KTP, SIM, Passport, NPWP, KK & Invoice) — secure, rate-limited, observable, and production-ready.**

The interesting part is not OCR, but **everything around the API**: cryptographic key management, token-bucket rate limiting, non-blocking metering, distributed tracing, automated delivery, and sub-60-second rollbacks.

> **Synthetic data only.** Every identity in this README is fake. Read [`PRIVACY.md`](PRIVACY.md) and [`USE_POLICY.md`](USE_POLICY.md) before processing real KTP images. Never commit real KTP data, real keys, or DB dumps.

## Contents

- [Features](#features)
- [Quickstart](#quickstart)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Architecture](#architecture)
- [Benchmarks & Demo Consumers](#benchmarks--demo-consumers)
- [Project Structure](#project-structure)
- [Development](#development)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Security & Privacy](#security--privacy)
- [License](#license)

## Features

- **Document OCR (6 types)** (`POST /api/v1/ocr/{ktp,sim,passport,npwp,kk,invoice}`) behind a pluggable `OCREngine` — Google Gemini Flash in production, deterministic `MockOCREngine` for offline/CI.
- **Secure API keys** — SHA-256 hashed at rest, scoped (`ocr:read`, `ocr:write`, `usage:read`), instant revocation, plaintext shown once.
- **Dual identity** — Keycloak OIDC for humans (HttpOnly `Secure` `SameSite=Lax` cookies, zero tokens in browser storage) + API keys for machines; SpiceDB ReBAC for tenant/project authorization.
- **Traffic guardrails** — $O(1)$ in-memory token bucket + monthly quota, standard `X-RateLimit-*` / `Retry-After` headers, `Idempotency-Key` replay protection on OCR writes.
- **Privacy by design** — images stay in ephemeral RAM buffers, never touch disk; zero PII in logs (UU PDP No. 27/2022).
- **Observable** — OpenTelemetry traces, Prometheus RED metrics, Grafana dashboards, SLOs + alert rules.
- **Shippable** — distroless images, GitHub Actions with 5 security gates, Playwright E2E (`tests/e2e`), OpenTofu IaC, sub-60-second rollback drill, 5 incident post-mortems.

## Quickstart

Prerequisites: **Go 1.26** (minimum 1.22), **Docker & Compose**, **Git**. Optional for the portal: **Bun** + Node 20+.

```bash
git clone https://github.com/amirfaisalz/lensio.git
cd lensio
git config core.hooksPath .githooks
cp .env.example .env   # default OCR_PROVIDER=mock, no Gemini key needed
docker compose up -d postgres

# Run API + dashboard together (API :8080, dashboard :3000, Keycloak :8082)
./dev.sh
```

Verify:

```bash
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
```

Run your first OCR (mock engine, synthetic fixture included):

```bash
export LENSIO_API_KEY="lensio_live_xxx"  # create one via the dashboard key manager
curl -s http://localhost:8080/api/v1/ocr/ktp \
  -H "Authorization: Bearer $LENSIO_API_KEY" \
  -F "document=@tests/fixtures/synthetic/valid_ktp.jpg" | head -c 600
```

Full stack (Postgres, Keycloak, SpiceDB, Prometheus, Grafana):

```bash
docker compose up -d
# API http://localhost:8080 · Dashboard http://localhost:3000
# Grafana http://localhost:3000 · Prometheus http://localhost:9090
# Keycloak http://localhost:8082 · Adminer http://localhost:8081
```

> Coverage badges reflect the last measurement (core ~88.5%, repo ~87.5%). 100% is the aspiration in `AGENTS.md`, not the current state — the numbers above are the measured reality. Re-measure locally with `go test -race -cover ./...`.
>
> Coverage is not the safety net on its own: the authorization boundary has its own suite (`apps/api/internal/http/handlers/tenant_isolation_test.go`) because a fully green run once sat on top of a cross-tenant read/write hole.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | API listen port |
| `ENV` | `development` | `development` / `staging` / `production` |
| `DATABASE_URL` | `postgres://lensio:lensio_dev_password@localhost:5432/lensio?sslmode=disable` | PostgreSQL 16 connection string |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` (structured `slog` JSON) |
| `OCR_PROVIDER` | `mock` | `mock` (offline/CI fixtures) or `gemini_flash` / `gemini` (live Vision AI) |
| `GEMINI_API_KEY` | — | Required only when `OCR_PROVIDER=gemini_flash` |
| `GEMINI_MODEL` | `gemini-3.6-flash` | e.g. `gemini-3.6-flash`, `gemini-2.5-flash`, `gemini-1.5-flash` |
| `KEYCLOAK_JWKS_URL` | — | Keycloak OIDC certs, e.g. `http://localhost:8082/realms/lensio/protocol/openid-connect/certs` |
| `KEYCLOAK_ISSUER` | derived from JWKS URL | OIDC issuer; auto-derived when empty |
| `KEYCLOAK_AUDIENCE` | — | Expected `aud` claim for human JWTs |
| `SPICEDB_ENDPOINT` | — | SpiceDB v1 REST, e.g. `http://localhost:50051`; empty = `MockAuthorizer` |
| `SPICEDB_PRESHARED_KEY` | — | Bearer key for SpiceDB check/write/schema |
| `SESSION_SECRET` | dev fallback | HS256 secret for HttpOnly `lensio_session` cookies |

Staging/production secrets are Container Apps secrets provisioned by OpenTofu (see [`infra/live/README.md`](infra/live/README.md)). Dev-only credentials in `docker-compose.yml` must never leave your laptop.

## API Reference

Full contract: [`openapi/openapi.yaml`](openapi/openapi.yaml).

| Method & Path | Auth | Description |
|---|---|---|
| `GET /health` | none | Liveness probe |
| `GET /ready` | none | Readiness probe (PG pool; `503` stops routing without crash-loop) |
| `POST /api/v1/ocr/ktp` | Bearer `lensio_live_xxx` (`ocr:write`) | KTP OCR, multipart `document`, max 5MB |
| `POST /api/v1/ocr/sim` | Bearer `lensio_live_xxx` (`ocr:write`) | SIM OCR, multipart `document`, max 5MB |
| `POST /api/v1/ocr/passport` | Bearer `lensio_live_xxx` (`ocr:write`) | Passport OCR, multipart `document`, max 5MB |
| `POST /api/v1/ocr/npwp` | Bearer `lensio_live_xxx` (`ocr:write`) | NPWP OCR, multipart `document`, max 5MB |
| `POST /api/v1/ocr/kk` | Bearer `lensio_live_xxx` (`ocr:write`) | KK OCR, multipart `document`, max 5MB |
| `POST /api/v1/ocr/invoice` | Bearer `lensio_live_xxx` (`ocr:write`) | Invoice OCR, multipart `document`, max 5MB |
| `GET /metrics` | — | Prometheus RED metrics |

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

Successful response (`200 OK`) — **SYNTHETIC EXAMPLE DATA**:

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

Request lifecycle (`POST /api/v1/ocr/ktp`, Bearer `lensio_live_xxx`, multipart `document`, max 5MB) — full sequence diagram in [`docs/architecture.md`](docs/architecture.md):

1. **Auth & guardrails** — `request_id` + OTel span → SHA-256 lookup + `ocr:write` scope → per-identity token bucket ($O(1)$) → monthly quota check.
2. **In-memory OCR** — image stays in RAM (`[]byte`, never disk) → magic-byte validation → `OCREngine` (Gemini/Mock) → deterministic NIK/SIM validation.
3. **Response** — `200 OK` JSON (<2,000ms SLA), buffer released immediately (UU PDP compliance).
4. **Async telemetry** — usage event → buffered channel → background PG insert; Prometheus metrics + trace close.

Key lifecycle: dashboard generates key → SHA-256 stored (masked prefix only) → plaintext shown **once**, never persisted. Probes: `GET /health` (liveness) vs `GET /ready` (PG pool, 503 stops routing without crash-loop).

## Architecture

```text
Authentication              Reliability              Security
• Scoped API Keys           • Token Bucket Limits    • Ephemeral Buffers (No PII)
• SHA-256 Hashing           • Monthly Quotas         • 5 Automated Security Gates
• Instant Revocation        • Async Metering         • Append-Only Audit Trail
```

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

| Layer | Components & Roles |
|---|---|
| **Backend** | Go 1.22+ (`apps/api`), native `net/http.ServeMux`, SHA-256 API keys, $O(1)$ token bucket, buffered-channel metering; PostgreSQL 16 via `pgx/v5/stdlib` + `golang-migrate`; Keycloak OIDC (human SSO) + SpiceDB ReBAC (tenant isolation) |
| **Vision AI** | Google Gemini 2.0 Flash Vision behind pluggable `OCREngine`; deterministic `MockOCREngine` for offline/CI testing |
| **Portal** | React 19 + TypeScript SPA (`apps/dashboard`), Vite + Tailwind, Vitest + Testing Library |
| **Telemetry** | OpenTelemetry Go SDK (W3C traces, `trace_id`/`request_id` correlation), Prometheus RED metrics, Grafana dashboards |
| **Cloud & Edge** | Cloudflare (DNS, DDoS, TLS 1.3, WAF) → Azure Container Apps (KEDA, Envoy, revision rollback); Container Apps secrets; OpenTofu IaC |
| **Security & CI** | Distroless images; Gitleaks, govulncheck, gosec, Trivy; 4-layer pre-commit gate (build → lint → `go test -race` → PII/Big-O audit) |

<details>
<summary><strong>Production engineering evidence matrix (PRD Section 38)</strong></summary>

| Domain | Lensio Production Evidence | Reference |
|---|---|---|
| **Language & Runtime** | Go 1.22+ clean architecture, native `net/http.ServeMux`, sub-80ms startup | [`ADR-001`](docs/decisions/ADR-001-why-go-for-api-and-ocr-service.md) |
| **Relational Database** | PostgreSQL 16 migrations (`golang-migrate`), `database/sql` + `pgx/v5/stdlib` | [`ADR-002`](docs/decisions/ADR-002-database-schema-and-api-key-hashing-strategy.md) |
| **Developer Portal** | React 19 + TypeScript SPA, type-safe API SDK, Tailwind CSS | [`apps/dashboard`](apps/dashboard) |
| **Cryptographic Security** | SHA-256 one-way API key hashing, zero plaintext secrets in storage | [`docs/security.md`](docs/security.md) |
| **Traffic Shaping** | $O(1)$ in-memory token bucket + monthly quota enforcer | [`ADR-003`](docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md), [`ADR-006`](docs/decisions/ADR-006-multi-instance-rate-limiting-tradeoffs.md) |
| **Telemetry & Observability** | OpenTelemetry Go SDK, Prometheus RED metrics, Grafana dashboards | [`docs/observability.md`](docs/observability.md) |
| **Data Privacy & Compliance** | Ephemeral memory-only image handling, zero PII logs (UU PDP No. 27/2022) | [`ADR-005`](docs/decisions/ADR-005-data-minimization-and-pii-protection-in-ocr-pipelines.md) |
| **Infrastructure as Code** | OpenTofu modules & one live stack with per-environment tfvars (Staging/Production) | [`infra/`](infra/) |
| **Cloud Hosting** | Azure Container Apps with KEDA autoscaling and Envoy ingress | [`ADR-004`](docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md) |
| **Continuous Delivery** | GitHub Actions with 5 security scanners (Gitleaks, govulncheck, gosec, Trivy) | [`.github/workflows`](.github/workflows/) |
| **Automated Rollbacks** | Immutable container revisions, sub-60-second traffic shifting drill | [`docs/rollback.md`](docs/rollback.md) |
| **Failure Resilience** | Documented post-mortem drills for 5 major production outage scenarios | [`docs/incidents`](docs/incidents/) |
| **Load Testing & Capacity** | Empirical k6: 1,000 RPS sustained, $P_{95} = 1.58\text{ms}$, 0 errors | [`docs/benchmarks/load-test-report.md`](docs/benchmarks/load-test-report.md) |
| **External Consumers** | Independent client apps consuming Lensio API (VeriForm, RentEase) | [`examples/`](examples/) |

</details>

## Benchmarks & Demo Consumers

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

## Project Structure

```text
lensio/
├── apps/api/            # Go REST API (handlers, middleware, services)
├── apps/dashboard/      # React 19 + TypeScript developer portal (Vite, Tailwind)
├── services/ocr/        # Pluggable OCREngine (Gemini adapter, mock, circuit breaker)
├── infra/               # OpenTofu modules, live stack, Keycloak realm, SpiceDB schema, observability
├── openapi/             # OpenAPI 3.0.3 contract (openapi.yaml) + viewers
├── tests/               # Unit, integration, E2E (Playwright), load (k6), synthetic fixtures only
├── examples/            # VeriForm + RentEase demo consumers
├── docs/                # Architecture, security, deployment, ADRs, incidents, benchmarks
└── scripts/             # dev.sh, load tests, demo runners, rollback/smoke drills
```

## Development

```bash
go build ./...              # Layer 1: compile
go vet ./...                # Layer 2: vet
golangci-lint run ./...     # Layer 2: lint
go test -race -cover ./...  # Layer 3: strict tests with race detector

cd apps/dashboard && bun install
bun run typecheck           # tsc --noEmit
bun run lint                # biome check
bun run test                # vitest run
```

Every commit runs the 4-layer pre-commit gate automatically (`.githooks/pre-commit`: build → lint → `go test -race` → PII/Big-O audit). Never use `--no-verify`. See [`AGENTS.md`](AGENTS.md) for the five mandatory operating rules.

Useful scripts: `./scripts/dev.sh` (API + dashboard), `./scripts/run-load-tests.sh` (k6), `./scripts/run-demo-consumers.sh` (VeriForm + RentEase), `./scripts/smoke-test.sh`, `./scripts/rollback.sh`.

## Documentation

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

## Contributing

Contributions are welcome! Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`AGENTS.md`](AGENTS.md) first — the five mandatory rules (minimal diffs, real tests, no PII, zero lint errors, pre-commit gates) apply to every change.

1. Fork and branch from `main` (`feat/...`, `fix/...`, `docs/...`).
2. Enable hooks: `git config core.hooksPath .githooks`.
3. One PR = one logical change, with verification evidence (build/lint/test output).
4. Update affected docs and `CHANGELOG.md` (`Unreleased`).

Roadmap (post-MVP, deferred): document verification, identity verification. All six OCR document types (KTP, SIM, Passport, NPWP, KK, Invoice) are live — please don't PR new document types without discussing first. Progress: [`DEVELOPMENT_TRACKING.md`](DEVELOPMENT_TRACKING.md).

## Security & Privacy

- Report vulnerabilities privately per [`SECURITY.md`](SECURITY.md) — never via public issues.
- Privacy commitments (UU PDP No. 27/2022): [`PRIVACY.md`](PRIVACY.md). Acceptable use: [`USE_POLICY.md`](USE_POLICY.md).

## License

Distributed under the [MIT License](LICENSE).
Copyright © 2026 Lensio Contributors.
