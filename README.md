# NusaID

> **Developer-first Indonesian KTP OCR API as a Service.**  
> Built as a production-grade API platform demonstrating end-to-end platform engineering: from authentication and rate limiting to automated delivery, observability, security, and instant rollback.

---

## The Vision & Core Principle

> **"Build the smallest real product that forces us to solve real production engineering problems."**

NusaID is not just an OCR demo. It is a full-lifecycle API product built to solve the operational realities of modern software engineering:
- How to securely ingest sensitive identity documents without storing raw PII.
- How to authenticate clients via cryptographic API keys with scoped permissions.
- How to meter usage, enforce tiered rate limits, and isolate tenant quotas.
- How to achieve sub-second health checks, distributed tracing, and zero-downtime rollbacks.

```text
               ┌────────────────┐
               │    Clients     │
               └───────┬────────┘
                       │ HTTPS / Bearer Token
                       ▼
               ┌────────────────┐
               │   Cloudflare   │ (WAF, DDoS, Edge SSL)
               └───────┬────────┘
                       │
                       ▼
        ┌──────────────────────────────┐
        │     Azure Container Apps     │
        │  ┌────────────────────────┐  │
        │  │      Go REST API       │  │
        │  │  - Auth & Scopes       │  │
        │  │  - Rate Limit & Quota  │  │
        │  │  - Pluggable OCREngine │  │
        │  └───────┬──────────┬─────┘  │
        └──────────┼──────────┼────────┘
                   │          │
         ┌─────────▼──┐    ┌──▼──────────────────┐
         │ PostgreSQL │    │ Vision OCR Engine   │
         │ (State)    │    │ (Gemini Flash/Mock) │
         └────────────┘    └─────────────────────┘
```

---

## Core API Specification

### Extract KTP Document

```http
POST /api/v1/ocr/ktp
Authorization: Bearer fg_live_xxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: multipart/form-data

document=@ktp-sample.jpg
```

#### Example Response (`200 OK`)

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

Every non-2xx response follows a predictable schema:

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "API rate limit exceeded. Please retry after 42 seconds.",
    "request_id": "req_01JABC123"
  }
}
```

---

## Technology Stack

| Domain | Technology | Rationale |
|---|---|---|
| **Backend API** | Go 1.22+ | Exceptional concurrency, ultra-low memory footprint, fast cold starts |
| **Relational Database** | PostgreSQL 16 | ACID transactions, relational integrity, migration tracking |
| **Developer Dashboard** | React 18 + TypeScript | Type-safe portal for key management, metrics, and interactive console |
| **Vision AI / OCR** | Google Gemini 2.0 / 1.5 Flash | SOTA multimodal extraction on Indonesian documents with JSON schema enforcement |
| **Test Engine** | MockOCREngine | Offline, deterministic fixtures for 100% test reliability in CI |
| **Observability** | OpenTelemetry + Grafana | Distributed tracing across HTTP and DB spans, RED metrics |
| **Infrastructure** | Azure Container Apps + Cloudflare | Containerized serverless scale-to-zero with edge security |
| **IaC** | OpenTofu + Terragrunt | Modular, open-source cloud infrastructure definitions |
| **Delivery Pipeline** | GitHub Actions | Automated security scanning (Gitleaks, govulncheck, gosec, trivy) and deployment |
| **E2E Testing** | Playwright | Full-browser verification of developer workflows |

---

## Security & Privacy Non-Negotiables

1. **Zero Permanent Image Storage**: Uploaded KTP images are processed in ephemeral memory buffers and discarded immediately. Raw documents are never saved to disk or databases.
2. **Zero PII in Application Logs**: Log outputs are scrubbed of NIK, names, dates of birth, and addresses. Correlated via `request_id` and `trace_id` only.
3. **Cryptographic Key Hashing**: Only SHA-256 hashes of API keys are stored in PostgreSQL. Raw secret keys (`fg_live_...`) are shown once upon creation.
4. **Deterministic Validation**: Extracted NIKs undergo strict 16-digit numeric and regional code validation before responses are returned.

---

## Repository Structure

```text
nusaid/
├── apps/
│   ├── api/             # Go REST API service
│   │   ├── cmd/server/  # Application entrypoint
│   │   ├── internal/    # Private packages (auth, ratelimit, quota, usage)
│   │   └── migrations/  # golang-migrate SQL scripts
│   └── dashboard/       # React + TypeScript developer portal
├── services/
│   └── ocr/             # OCREngine interface & provider adapters (Gemini, Mock)
├── openapi/             # OpenAPI 3.0 specification (`openapi.yaml`)
├── tests/
│   ├── fixtures/        # Synthetic, redacted test KTPs (NO REAL PII)
│   ├── integration/     # Go database & service integration tests
│   └── e2e/             # Playwright browser end-to-end tests
├── infra/               # OpenTofu modules & Terragrunt live environments
├── .agents/             # AI agent operational guidelines
├── .githooks/           # Pre-commit 4-layer verification gate
├── AGENTS.md            # Agent operational rules & boundaries
├── DEVELOPMENT_CHECKLIST.md # Granular progress tracking
└── PRD.md               # Product Requirements Document
```

---

## Local Development Quickstart

### Prerequisites
- Go 1.22+
- Docker & Docker Compose
- Git

### 1. Clone & Configure Git Hooks
```bash
git clone https://github.com/amirfaisalz/nusaid.git
cd nusaid
git config core.hooksPath .githooks
```

### 2. Launch Local Environment
```bash
docker compose up -d
```

### 3. Verify Health Probes
```bash
# Liveness probe
curl -i http://localhost:8080/health

# Readiness probe (verifies database connectivity)
curl -i http://localhost:8080/ready
```

---

## Pre-Commit Verification Gate

Every commit is strictly verified by `.githooks/pre-commit` against four layers:

```text
Layer 1: Type Checking & Compilation (go build ./..., tsc --noEmit)
Layer 2: Linting & Code Quality (golangci-lint run, go vet)
Layer 3: Strict Tests & 100% Coverage Target (go test -race -cover ./...)
Layer 4: Big O Sanity & Security Guardrails (PII leak audit & scratch hygiene)
```

Bypassing git hooks (`--no-verify`) is prohibited.

---

## Future Product Expansion Roadmap

While the active MVP is strictly focused on **Indonesian KTP OCR**, the platform architecture is designed to scale into a comprehensive document suite:

```text
NusaID Product Ecosystem
│
├── Document OCR Expansion
│   ├── [x] KTP OCR (Active MVP Focus)
│   ├── [ ] SIM OCR (Driver's License)
│   ├── [ ] Passport OCR
│   ├── [ ] NPWP OCR (Tax ID)
│   ├── [ ] KK OCR (Kartu Keluarga)
│   └── [ ] Invoice OCR
│
├── Verification & Trust Services
│   ├── [ ] Document Verification (Tampering / Forgery Detection)
│   └── [ ] Identity Verification (Facial Matching / Liveness)
│
└── Intelligent Extraction
    └── [ ] AI Document Extraction (Semi-structured vision parsing)
```

---

## License

Distributed under the [MIT License](LICENSE).  
Copyright © 2026 NusaID Contributors.
