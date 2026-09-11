# NusaID Deployment & Environment Promotion Runbook

> Operational guide for building, packaging, deploying, and promoting NusaID services across environments.

---

## 1. Pipeline Overview

NusaID follows a trunk-based continuous delivery model with automated quality gates and containerized immutable artifacts:

```text
[Feature Branch / PR]
       │
       ▼
  .github/workflows/ci.yml (Go race tests, frontend vitest/lint, OpenAPI spectral, Postgres integration)
  .github/workflows/security.yml (Gitleaks, govulncheck, gosec, Trivy IaC & container)
       │
       ▼ Merge to main
  .github/workflows/release-build.yml (Multi-platform Docker build -> ghcr.io/amirfaisalz/nusaid-api:<sha>)
       │
       ▼ Automated
  .github/workflows/deploy.yml (Deploy to Staging -> scripts/smoke-test.sh)
       │
       ▼ Release Tag (v*) or Manual Approval
  .github/workflows/deploy.yml (Deploy to Production -> Production smoke-test -> Gated rollback)
```

---

## 2. Environments

NusaID maintains three isolated environments:

| Environment | Purpose | Ingress URL | Database Tier | Scaling |
|---|---|---|---|---|
| **Development** | Local Docker Compose | `http://localhost:8080` | Local Postgres 16 container | 1 replica |
| **Staging** | Pre-production validation | `https://api.staging.nusaid.com` | Flexible Server (Standard B1ms) | 1-2 replicas |
| **Production** | Live consumer traffic | `https://api.nusaid.com` | Flexible Server (General Purpose HA) | 2-10 replicas |

---

## 3. Container Image Tagging Strategy

Every container image pushed to GitHub Container Registry (`ghcr.io`) is tagged immutably:
1. **Commit SHA**: `nusaid-api:sha-a1b2c3d` (exact build provenance).
2. **Semantic Version**: `nusaid-api:1.4.0` (production releases).
3. **Latest / Branch**: `nusaid-api:latest` and `nusaid-api:main` (staging preview).

---

## 4. Automated Security Gates

Before any deployment proceeds, the following automated scans must return **zero high or critical findings**:

1. **Secret Scanning**: Gitleaks verifies that no private keys, database passwords, or Google AI Studio tokens are committed.
2. **Dependency Vulnerabilities**: `govulncheck` audits Go module call graphs, and `npm audit` checks frontend packages.
3. **Static Application Security Testing (SAST)**: `gosec` scans Go AST for code-level security issues.
4. **IaC Security**: Trivy inspects OpenTofu/Terragrunt definitions in `infra/` for overly permissive network rules.
5. **Container Vulnerability Scanning**: Built container layers are scanned using Trivy with `--severity HIGH,CRITICAL --exit-code 1`.

---

## 5. Automated Smoke Testing

Post-deployment validation is executed via `scripts/smoke-test.sh`:
- **Liveness Probe**: Verifies `/health` returns HTTP 200 within 3s.
- **Readiness Probe**: Verifies `/ready` confirms database connectivity.
- **Security Check**: Verifies `/api/v1/ocr/ktp` properly rejects unauthenticated requests (HTTP 401).
- **OCR Functional Test**: Sends a synthetic in-memory KTP image fixture to verify end-to-end extraction.
- **Dashboard Probe**: Verifies frontend web asset serving.

If the smoke test fails on production, the deployment pipeline halts and triggers automated revision rollback.

---

## 6. Related Runbooks & Documentation

- **Emergency Rollback Runbook**: [`docs/rollback.md`](./rollback.md) (Under-60-second recovery procedures)
- **System Architecture**: [`docs/architecture.md`](./architecture.md) (C4 diagrams and container topologies)
- **Security & Privacy Architecture**: [`docs/security.md`](./security.md) (Scanning gates and secret controls)
- **Observability Runbook**: [`docs/observability.md`](./observability.md) (Telemetry, metrics, and SLO definitions)

