# Lensio KTP OCR API

> Affordable Indonesian KTP OCR API for developers and businesses.

Lensio is a production-oriented API platform that provides Indonesian KTP OCR as a service.

Developers can integrate KTP document extraction into their applications through a simple REST API without having to build, operate, secure, monitor, and scale their own OCR infrastructure.

The project is intentionally designed as both:

1. A usable API product
2. A production engineering portfolio demonstrating end-to-end ownership of an API platform

---

# 1. Product Overview

## Problem

Companies building onboarding, rental, fintech, insurance, HR, marketplace, and other applications may need to extract structured information from Indonesian KTP documents.

Building this capability internally requires:

- OCR infrastructure
- Image preprocessing
- Document classification
- Field extraction
- Validation
- API authentication
- Rate limiting
- Usage tracking
- Monitoring
- Deployment infrastructure
- Security controls
- Failure recovery

For many companies, this is unnecessary engineering overhead.

## Solution

Lensio provides a simple API:

```http
POST /api/v1/ocr/ktp
```

A client uploads a KTP image.

Lensio processes the document and returns structured information.

Example:

```json
{
  "id": "ocr_01JABC123",
  "status": "completed",
  "document_type": "ktp",
  "confidence": 0.96,
  "data": {
    "nik": "REDACTED",
    "nama": "BUDI SANTOSO",
    "tempat_lahir": "BANDAR LAMPUNG",
    "tanggal_lahir": "1990-05-12",
    "jenis_kelamin": "LAKI-LAKI",
    "alamat": "REDACTED",
    "rt_rw": "001/002",
    "kelurahan": "SUKARAME",
    "kecamatan": "SUKARAME",
    "agama": "ISLAM",
    "status_perkawinan": "KAWIN",
    "pekerjaan": "KARYAWAN SWASTA",
    "kewarganegaraan": "WNI"
  },
  "processing": {
    "latency_ms": 1820
  }
}
```

The actual demo environment must use synthetic or redacted documents rather than real personal identity documents.

---

# 2. Product Vision

Lensio should feel like a small developer-first API company.

The developer experience should be:

```text
Create account
      ↓
Create API key
      ↓
Read API documentation
      ↓
Send KTP image
      ↓
Receive structured JSON
      ↓
Monitor usage
      ↓
Manage quota
```

The goal is not to create a massive OCR company.

The goal is to demonstrate how to build and operate a production-grade API product.

---

# 3. Example Customers

Lensio's API can be consumed by applications such as:

## VeriForm

A fictional identity onboarding platform.

Use case:

```text
User uploads KTP
       ↓
VeriForm
       ↓
Lensio OCR API
       ↓
Structured identity data
       ↓
Registration continues
```

## RentEase

A fictional vehicle rental platform.

Use case:

```text
Customer submits KTP
       ↓
RentEase sends image to Lensio
       ↓
KTP fields extracted
       ↓
Rental verification workflow continues
```

These applications are deliberately separate consumers.

This demonstrates that Lensio is an API product rather than an application with an internal OCR endpoint.

---

# 4. Core API

## OCR

```http
POST /api/v1/ocr/ktp
```

Request:

```http
Authorization: Bearer lensio_live_xxxxx
Content-Type: multipart/form-data
```

Form:

```text
document=<image>
```

Response:

```json
{
  "id": "ocr_01JABC123",
  "status": "completed",
  "document_type": "ktp",
  "confidence": 0.96,
  "data": {
    "nik": "REDACTED",
    "nama": "BUDI SANTOSO",
    "tanggal_lahir": "1990-05-12"
  },
  "processing": {
    "latency_ms": 1820
  }
}
```

---

# 5. API Endpoints

## Authentication

```http
POST /api/v1/auth/api-keys
GET /api/v1/auth/api-keys
DELETE /api/v1/auth/api-keys/:id
GET /api/v1/auth/verify
```

## OCR

```http
POST /api/v1/ocr/ktp
GET /api/v1/ocr/:id
```

## Usage

```http
GET /api/v1/usage
GET /api/v1/usage/daily
GET /api/v1/usage/endpoints
GET /api/v1/usage/records
```

## Account

```http
GET /api/v1/account
GET /api/v1/account/plan
PUT /api/v1/account/plan
GET /api/v1/account/members
```

## Health

```http
GET /health
GET /ready
```

## API Documentation

```text
/openapi
/docs
```

---

# 6. API Versioning

Lensio must support versioned APIs.

Example:

```text
/api/v1/ocr/ktp
/api/v2/ocr/ktp
```

Versioning must allow future schema changes without unexpectedly breaking existing consumers.

Deprecated versions should provide appropriate metadata.

Example:

```http
Deprecation: true
Sunset: 2027-01-01
```

Documentation must explain migration between versions.

---

# 7. API Authentication

Every production API request requires authentication.

Initial authentication mechanism:

```text
API Key
```

Example:

```http
Authorization: Bearer lensio_live_xxxxxxxxx
```

API keys must:

- Never be stored in plaintext
- Be hashed before persistence
- Support revocation
- Have an owner
- Have an environment
- Have scopes
- Have creation timestamps
- Have last-used timestamps

Example:

```text
lensio_live_xxxxxxxxx
```

Only the hashed representation is stored in PostgreSQL.

---

# 8. Authorization

Lensio should support scoped API keys.

Example scopes:

```text
ocr:read
ocr:write
usage:read
```

A key with only:

```text
ocr:read
```

must not be able to submit OCR requests.

Authorization should be enforced at the API layer.

---

# 9. Rate Limiting

Rate limiting protects the service from abuse and ensures fair usage.

Example:

```text
Free
10 requests/minute

Starter
30 requests/minute

Pro
100 requests/minute

Business
Custom
```

Example response:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 42
```

Response:

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "API rate limit exceeded"
  }
}
```

Rate limit headers should expose useful information.

Example:

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1726039200
```

---

# 10. Quotas

Rate limiting and quota are separate concepts.

Rate limit:

```text
How quickly can the customer send requests?
```

Quota:

```text
How many requests can the customer consume within their billing period?
```

Example:

```text
Pro Plan

Monthly quota
8,421 / 10,000

Remaining
1,579
```

When the quota is exhausted:

```http
HTTP 429
```

with an appropriate error code.

---

# 11. Usage Metering

Every OCR request should generate usage information.

Record:

```text
request_id
organization_id
api_key_id
endpoint
API version
timestamp
HTTP status
processing latency
OCR engine
document type
success/failure
```

Example:

```json
{
  "request_id": "req_01JABC",
  "endpoint": "/api/v1/ocr/ktp",
  "status": 200,
  "latency_ms": 1820,
  "timestamp": "2026-09-11T10:00:00Z"
}
```

Sensitive document contents must not be written to application logs.

---

# 12. OCR Architecture

The OCR system should use an abstraction layer.

```text
                    OCR Service
                         │
                  ┌──────┴──────┐
                  │ OCREngine    │
                  │ interface    │
                  └──────┬──────┘
                         │
            ┌────────────┼────────────┐
            ▼            ▼            ▼
       Mock Engine   Gemini Flash   Cloud / Local
      (Test Fixture)  (Default AI)   (Extensible)
```

Go interface:

```go
type OCREngine interface {
    Extract(ctx context.Context, image []byte) (*OCRResult, error)
}
```

This allows OCR providers to be replaced without changing the public API.

---

# 13. OCR Processing Pipeline

```text
Image Upload
     ↓
File Validation
     ↓
Image Preprocessing
     ↓
Document Classification
     ↓
OCR
     ↓
Text Extraction
     ↓
KTP Field Extraction
     ↓
Normalization
     ↓
Validation
     ↓
Confidence Calculation
     ↓
Structured JSON
```

The system should distinguish between:

- OCR failure
- Unsupported document
- Invalid image
- Low confidence extraction
- Internal processing failure

---

# 14. AI Usage

AI may be used as an extraction or normalization component.

Example:

```text
OCR raw text
      ↓
AI field extraction
      ↓
Schema validation
      ↓
Business validation
      ↓
API response
```

AI must not be treated as the only validation layer.

The application should use deterministic validation where possible.

For example:

```text
NIK
├── Expected length
├── Numeric validation
└── Format validation
```

AI output must be validated against a strict schema before being returned.

---

# 15. Security and Privacy

KTP contains sensitive personal information.

Security is therefore a first-class feature.

## Data minimization

Uploaded images should not be retained indefinitely.

Default architecture:

```text
Upload
  ↓
Temporary processing
  ↓
OCR
  ↓
Structured result
  ↓
Temporary image deleted
```

Retention should be configurable.

## Secrets

Never commit:

- API keys
- Cloud credentials
- OCR provider credentials
- Database passwords

## API Keys

Only hashed API keys are stored.

## Logs

Logs must not contain:

- NIK
- full name
- address
- date of birth
- raw KTP images

Use request IDs instead.

## Security scanning

CI should include:

```text
Secret scanning
Dependency scanning
SAST
Container scanning
IaC scanning
```

High or critical findings should fail the pipeline according to project policy.

---

# 16. Observability

Lensio must be observable in production.

Use:

```text
OpenTelemetry
Grafana
```

Track:

### Availability

```text
Request success rate
```

### Performance

```text
P50 latency
P95 latency
P99 latency
```

### Errors

```text
4xx
5xx
OCR failures
Provider failures
```

### Rate limiting

```text
429 responses
```

### Business metrics

```text
OCR requests
Successful OCR
Failed OCR
Average processing time
```

---

# 17. Health Checks

Two health endpoints:

```http
GET /health
```

Indicates whether the application process is alive.

```http
GET /ready
```

Indicates whether the application is ready to serve traffic.

Readiness should check required dependencies such as PostgreSQL.

---

# 18. Dashboard

Lensio provides a React + TypeScript dashboard.

Main navigation:

```text
Overview
API Keys
API Documentation
Usage
Requests
Account
Settings
```

## Overview

Display:

```text
Requests
Success Rate
Error Rate
P95 Latency
Quota Usage
Rate Limit Violations
```

Example:

```text
Lensio

Requests       12,842
Success         98.7%
Errors           1.3%
P95 latency       1.8s

Quota
8,421 / 10,000
```

---

# 19. Developer Experience

The developer should be able to make the first API request quickly.

Example:

```bash
curl -X POST \
  https://api.Lensio.dev/api/v1/ocr/ktp \
  -H "Authorization: Bearer lensio_live_xxxxx" \
  -F "document=@ktp-test.jpg"
```

Response:

```json
{
  "id": "ocr_01JABC",
  "status": "completed",
  "confidence": 0.96,
  "data": {
    "nama": "BUDI SANTOSO"
  }
}
```

The documentation should include:

- Quick start
- Authentication
- API keys
- OCR API
- Errors
- Rate limits
- Quotas
- Versioning
- Usage
- SDK examples

---

# 20. Error Model

All API errors should have a predictable schema.

```json
{
  "error": {
    "code": "invalid_document",
    "message": "The uploaded document could not be processed",
    "request_id": "req_01JABC"
  }
}
```

Example error codes:

```text
invalid_request
invalid_api_key
insufficient_scope
rate_limit_exceeded
quota_exceeded
invalid_document
unsupported_document
ocr_failed
low_confidence
internal_error
```

---

# 21. PostgreSQL Data Model

Core tables:

```text
organizations
users
api_keys
plans
usage_records
ocr_requests
audit_logs
```

Possible relationships:

```text
Organization
   │
   ├── Users
   ├── API Keys
   ├── OCR Requests
   └── Usage Records
```

API keys belong to an organization.

OCR requests belong to an organization and API key.

---

# 22. Infrastructure

Initial production architecture:

```text
                    Internet
                       │
                       ▼
                   Cloudflare
                       │
                       ▼
               Azure Container Apps
                       │
              ┌────────┴────────┐
              ▼                 ▼
          Go API            Dashboard
              │
       ┌──────┴──────┐
       ▼             ▼
 PostgreSQL      OCR Provider
```

Infrastructure as Code:

```text
OpenTofu
Terragrunt
```

Containerization:

```text
Docker
```

No Kubernetes.

The goal is to demonstrate appropriate infrastructure rather than unnecessary complexity.

---

# 23. Environments

Three environments:

```text
development
staging
production
```

Deployment flow:

```text
Developer
    ↓
Pull Request
    ↓
CI
    ↓
Tests
    ↓
Security Scans
    ↓
Build
    ↓
Staging
    ↓
Smoke Tests
    ↓
Production
```

---

# 24. CI/CD

GitHub Actions should execute:

```text
Go tests
Go lint
Frontend tests
Playwright E2E
Dependency scanning
Secret scanning
SAST
Docker build
Container scanning
IaC validation
OpenAPI validation
```

Example pipeline:

```text
Pull Request
     │
     ├── Unit Tests
     ├── Integration Tests
     ├── Playwright
     ├── Security
     ├── Lint
     └── Build
          │
          ▼
      Quality Gate
          │
          ▼
       Merge
          │
          ▼
    Build Container
          │
          ▼
       Staging
          │
          ▼
     Smoke Tests
          │
          ▼
     Production
```

---

# 25. Rollback

Every deployment must produce a versioned artifact.

Example:

```text
Lensio-api:1.4.0
Lensio-api:1.4.1
```

If production deployment fails:

```text
1.4.1
 ↓
Health degradation
 ↓
Rollback
 ↓
1.4.0
```

Rollback must be documented and reproducible.

---

# 26. Failure Scenarios

The project should intentionally simulate failures.

## Scenario A: OCR provider failure

```text
Lensio
   ↓
OCR provider
   X
timeout
```

Expected behavior:

- timeout
- retry according to policy
- return controlled error
- emit telemetry
- do not expose provider credentials

## Scenario B: Database unavailable

```text
API
 ↓
PostgreSQL
 X
```

Expected:

```text
/ready → unhealthy
```

Traffic should not be routed to an instance that cannot serve required requests.

## Scenario C: Deployment failure

```text
Production deployment
        ↓
Smoke test
        X
```

Expected:

```text
Deployment stopped
```

## Scenario D: Production regression

```text
5xx
0.2%
 ↓
4.8%
```

Expected:

```text
Alert
 ↓
Investigate
 ↓
Rollback
```

---

# 27. AI Agent Development Workflow

Because the target role emphasizes AI-assisted engineering, the repository should document an agent workflow.

Example:

```text
Issue
 ↓
AI agent reads architecture rules
 ↓
Implementation
 ↓
Tests
 ↓
Security checks
 ↓
Review
 ↓
Pull Request
 ↓
CI
```

Repository:

```text
.agents/
├── architecture.md
├── coding.md
├── testing.md
├── security.md
└── deployment.md
```

Agents should not be allowed to bypass:

- tests
- security checks
- CI
- review
- deployment gates

The objective is to demonstrate controlled AI-assisted development rather than uncontrolled code generation.

---

# 28. Testing Strategy

## Unit tests

Go:

```text
API handlers
authentication
authorization
rate limiting
quota
OCR parsing
validation
```

Frontend:

```text
components
hooks
API client
dashboard logic
```

## Integration tests

Test:

```text
API → PostgreSQL
API → OCR service
API key → authorization
quota → usage
```

## E2E

Playwright should test:

```text
Login
Create API key
Upload document
Receive OCR result
View usage
Revoke API key
```

---

# 29. OpenAPI

The public API must have an OpenAPI specification.

Location:

```text
/openapi/openapi.yaml
```

The specification should define:

- endpoints
- authentication
- request schemas
- response schemas
- errors
- rate limits
- examples

OpenAPI should be validated in CI.

---

# 30. Pricing Simulation

Lensio can demonstrate API-as-a-product concepts.

Example plans:

| Plan     | OCR Requests | Rate Limit |
| -------- | -----------: | ---------: |
| Free     |    100/month |     10/min |
| Starter  |  1,000/month |     30/min |
| Pro      | 10,000/month |    100/min |
| Business |       Custom |     Custom |

These prices are illustrative and do not represent actual commercial pricing.

The objective is to demonstrate:

```text
Customer
 ↓
Plan
 ↓
Quota
 ↓
Usage
 ↓
API access
```

---

# 31. Repository Structure

```text
Lensio/
│
├── apps/
│   ├── api/
│   │   ├── cmd/
│   │   ├── internal/
│   │   │   ├── auth/
│   │   │   ├── authorization/
│   │   │   ├── ocr/
│   │   │   ├── ratelimit/
│   │   │   ├── quota/
│   │   │   ├── usage/
│   │   │   ├── audit/
│   │   │   └── observability/
│   │   └── migrations/
│   │
│   └── dashboard/
│
├── services/
│   └── ocr/
│
├── openapi/
│   └── openapi.yaml
│
├── tests/
│   ├── integration/
│   └── e2e/
│
├── infra/
│   ├── modules/
│   └── live/
│       ├── staging/
│       └── production/
│
├── .github/
│   └── workflows/
│
├── .agents/
│   ├── architecture.md
│   ├── coding.md
│   ├── testing.md
│   ├── security.md
│   └── deployment.md
│
├── docs/
│   ├── architecture.md
│   ├── security.md
│   ├── deployment.md
│   ├── rollback.md
│   ├── observability.md
│   ├── incidents/
│   └── decisions/
│
├── docker-compose.yml
├── Dockerfile
├── README.md
└── LICENSE
```

---

# 32. Technology Stack

## Backend

```text
Go
PostgreSQL
REST
OpenAPI
```

## Frontend

```text
React
TypeScript
```

## Authentication

```text
Keycloak
OIDC
JWT
API Keys
```

## OCR

```text
Pluggable OCR Engine:
- Default Vision AI: Google Gemini 2.0 / 1.5 Flash (Google AI Studio Free Tier)
- Test Engine: MockOCREngine (Deterministic fixtures for CI)
```

## Observability

```text
OpenTelemetry
Grafana
```

## Infrastructure

```text
Azure
Azure Container Apps
Cloudflare
OpenTofu
Terragrunt
```

## Delivery

```text
GitHub Actions
Docker
Playwright
```

## Security

```text
Secret scanning
Dependency scanning
SAST
Container scanning
IaC scanning
```

---

# 33. Non-Goals

To prevent overengineering, Lensio will NOT initially include:

- Kubernetes
- Kafka
- Complex distributed event systems
- Full billing infrastructure
- Payment processing
- Mobile applications
- Facial recognition
- Biometric verification
- Government database integration
- Real KTP storage
- Large-scale multi-region architecture
- Dozens of OCR document types

The first version focuses on one document type:

```text
Indonesian KTP
```

and one core API:

```text
POST /api/v1/ocr/ktp
```

---

# 34. MVP

The minimum viable product must demonstrate:

## Product

- KTP OCR
- structured JSON response
- API documentation
- API key authentication
- developer dashboard
- usage tracking

## Platform

- rate limiting
- quotas
- API versioning
- audit logs
- health checks

## Engineering

- Go
- PostgreSQL
- React + TypeScript
- Docker
- GitHub Actions
- Playwright
- OpenAPI

## Production

- staging
- production
- observability
- security scanning
- deployment
- rollback

---

# 35. Definition of Done

Lensio MVP is considered complete when:

- A developer can create an account
- A developer can create an API key
- A developer can read the API documentation
- A developer can submit a synthetic KTP image
- The API returns structured OCR data
- Invalid documents return predictable errors
- API keys are securely stored
- Rate limits work
- Quotas work
- Usage is recorded
- API requests are observable
- Logs do not expose sensitive KTP data
- CI validates code and security
- E2E tests run automatically
- Docker images can be built reproducibly
- Staging deployment works
- Production deployment works
- Failed deployment can be rolled back
- Architecture is documented
- Security decisions are documented
- Failure scenarios are documented
- A separate demo consumer can successfully consume the API

---

# 36. Seven-Day Implementation Plan

## Day 1: Foundation

Build:

```text
Go API
PostgreSQL
React dashboard
Docker Compose
OpenAPI
```

Implement:

```text
/health
/ready
```

---

## Day 2: API Product

Implement:

```text
API keys
Authentication
Authorization
OCR endpoint
Error model
Versioning
```

---

## Day 3: OCR Pipeline

Implement:

```text
Image validation
OCR abstraction
OCR provider
KTP extraction
Validation
Confidence score
```

Use only synthetic/redacted test documents.

---

## Day 4: API Platform

Implement:

```text
Rate limiting
Quotas
Usage tracking
Audit logs
Dashboard metrics
```

---

## Day 5: Production Infrastructure

Implement:

```text
Docker
GitHub Actions
Security scanning
OpenTelemetry
Grafana
Azure Container Apps
OpenTofu
Terragrunt
```

---

## Day 6: Reliability

Implement:

```text
Playwright
Smoke tests
Deployment promotion
Rollback
Failure simulations
Least privilege
```

---

## Day 7: Portfolio Quality

Finish:

```text
README
Architecture diagram
API documentation
Security documentation
Deployment documentation
Rollback documentation
ADR
Incident reports
DORA metrics methodology
AI agent workflow
Demo consumer
```

Record a short demo showing:

```text
Developer creates API key
        ↓
Client sends KTP
        ↓
OCR response
        ↓
Usage appears in dashboard
        ↓
Rate limit triggered
        ↓
Deployment
        ↓
Failure simulation
        ↓
Rollback
```

---

# 37. Portfolio Story

The project should be presented as:

> **Lensio is an affordable KTP OCR API built to explore what it takes to operate a production API as a product.**

The interesting part is not only OCR.

The engineering challenge is everything around the API:

```text
                ┌───────────────┐
                │   KTP OCR     │
                │    Product    │
                └───────┬───────┘
                        │
       ┌────────────────┼────────────────┐
       │                │                │
 Authentication      Reliability      Security
       │                │                │
 API Keys           Rate Limits       Scanning
 OIDC               Quotas            Secrets
 Authorization      Observability     Audit
       │                │                │
       └────────────────┼────────────────┘
                        │
                 Production Path
                        │
                CI → Staging
                        ↓
                   Production
                        ↓
                    Rollback
```

This is the core engineering story.

---

# 38. Production & Platform Engineering Relevance

Lensio intentionally demonstrates capabilities relevant to a modern platform/API engineering role.

| Requirement      | Lensio Evidence           |
| ---------------- | ------------------------- |
| Go               | Go production API         |
| PostgreSQL       | Persistent API/usage data |
| React            | Developer dashboard       |
| TypeScript       | Dashboard                 |
| API as a product | KTP OCR API               |
| Authentication   | API keys + OIDC           |
| Authorization    | API scopes                |
| Rate limits      | Request limits            |
| Quotas           | Usage plans               |
| Versioning       | `/v1`, `/v2`              |
| OpenAPI          | Public API contract       |
| Playwright       | E2E tests                 |
| Docker           | Containerized services    |
| GitHub Actions   | CI/CD                     |
| Azure            | Production infrastructure |
| OpenTofu         | Infrastructure as Code    |
| Terragrunt       | Environment management    |
| OpenTelemetry    | Application telemetry     |
| Grafana          | Observability             |
| Security         | Automated scanning        |
| Production       | Staging → production      |
| Rollback         | Versioned deployments     |
| AI agents        | Controlled agent workflow |

The project does not claim to replace real production experience.

It provides concrete evidence of the ability to understand and implement the engineering concepts required by the role.

---

# 39. The One-Sentence Explanation

If someone asks:

**"What is Lensio?"**

Answer:

> **Lensio is a developer-first KTP OCR API that lets applications extract structured Indonesian KTP data through a secure, rate-limited, observable and production-ready API.**

If they ask:

**"Why did you build it?"**

Answer:

> **I wanted to build more than an OCR feature. I wanted to understand the complete production path of an API product, from authentication and usage management to automated delivery, observability, security and rollback.**

---

# 40. Final Architecture

```text
                         ┌──────────────────────┐
                         │      Developers      │
                         └──────────┬───────────┘
                                    │
                              API Documentation
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      Lensio       │
                         │   Developer Portal   │
                         └──────────┬───────────┘
                                    │
                              API Key / OIDC
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │    Cloudflare        │
                         └──────────┬───────────┘
                                    │
                                    ▼
                   ┌────────────────────────────────┐
                   │       Azure Container Apps     │
                   │                                │
                   │          Go API                 │
                   │                                │
                   │  Auth                         │
                   │  Authorization                │
                   │  Rate Limit                   │
                   │  Quota                        │
                   │  Usage                        │
                   │  OCR                           │
                   │  OpenAPI                       │
                   └───────────────┬────────────────┘
                                   │
                 ┌─────────────────┼──────────────────┐
                 │                 │                  │
                 ▼                 ▼                  ▼
           PostgreSQL          OCR Engine       OpenTelemetry
                                                     │
                                                     ▼
                                                   Grafana


                     Delivery Pipeline

Developer
   │
   ▼
GitHub
   │
   ▼
GitHub Actions
   │
   ├── Tests
   ├── Playwright
   ├── Security
   ├── Dependency Scan
   ├── SAST
   ├── Container Scan
   └── IaC Validation
   │
   ▼
Docker Image
   │
   ▼
Staging
   │
   ▼
Smoke Tests
   │
   ▼
Production
   │
   └────── Failure ──────► Rollback
```

---

# 41. Project Principle

The most important principle for Lensio is:

> **Build the smallest real product that forces us to solve real production engineering problems.**

Do not build features merely to add technologies to the README.

Every technology should exist because the product needs it.

```text
KTP OCR
    ↓
Needs API
    ↓
Needs authentication
    ↓
Needs API keys
    ↓
Needs rate limiting
    ↓
Needs quotas
    ↓
Needs usage metering
    ↓
Needs observability
    ↓
Needs security
    ↓
Needs automated deployment
    ↓
Needs rollback
```

That is what turns Lensio from a portfolio CRUD project into a credible production engineering project.

---

# 42. Future Product Expansion Roadmap

While the MVP strictly focuses on **Indonesian KTP OCR** and establishing the core API product infrastructure, the platform is architected to expand into a comprehensive identity and document processing suite:

```text
Lensio Product Ecosystem
│
├── Document OCR Expansion
│   ├── KTP OCR (Current MVP Focus)
│   ├── SIM OCR (Surat Izin Mengemudi)
│   ├── Passport OCR (Indonesian & International Passports)
│   ├── NPWP OCR (Nomor Pokok Wajib Pajak)
│   ├── KK OCR (Kartu Keluarga)
│   └── Invoice OCR (Commercial Invoices & E-Faktur)
│
├── Verification & Trust Services
│   ├── Document Verification (Forgery & tampering detection)
│   └── Identity Verification (Facial matching & liveness checks)
│
└── Intelligent Extraction
    └── AI Document Extraction (Vision LLM extraction for custom/unstructured documents)
```

The pluggable `OCREngine` interface and modular API routing established in the MVP will allow adding these document types and services without re-architecting the platform core.

