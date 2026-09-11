# Deployment & Operations Guide

> Standards for containerization, environment promotion, OpenTofu IaC, and rollback operations.

---

## 1. Production Architecture Topology

```text
               Internet
                  │
                  ▼
              Cloudflare (DNS, SSL, DDoS)
                  │
                  ▼
          Azure Container Apps
                  │
         ┌────────┴────────┐
         ▼                 ▼
     Go API App     Dashboard App
         │
    ┌────┴──────────────────────────┐
    ▼                               ▼
PostgreSQL (Flexible Server)   OpenTelemetry Collector
                                    │
                                    ▼
                                 Grafana
```

---

## 2. Containerization Standards

- **Go API (`apps/api/Dockerfile`)**:
  - Build stage: `golang:1.22-alpine` or standard official image with cached dependencies.
  - Final runtime stage: Minimal `gcr.io/distroless/static-debian12` or `alpine` running as a non-privileged `nonroot` user (UID 65532).
  - Include CA certificates for TLS requests.
- **Dashboard (`apps/dashboard/Dockerfile`)**:
  - Build stage: `node:20-alpine`.
  - Final runtime stage: `nginx:alpine` serving pre-built static assets.

---

## 3. Environment Promotion Flow

```text
[Feature Branch]
       │ (PR Open)
       ▼
[CI Gates] (Lint + Test -race + Security Scanners + OpenAPI Validation)
       │ (Merge to main)
       ▼
[Build & Publish] (Tagged: lensio-api:<git-sha> & lensio-api:<semver>)
       │
       ▼
[Staging Deployment] (Automated deployment to Azure Container Apps Staging)
       │
       ▼
[Automated Smoke Tests] (Verify /health, /ready, and test OCR call)
       │ (Manual Approval / Release Tag)
       ▼
[Production Deployment] (Deploy to Azure Container Apps Production)
```

---

## 4. Rollback Policy & Procedure

Every deployment is immutable and uniquely versioned:
- Target format: `lensio-api:1.4.0` (SemVer) or `lensio-api:sha-a1b2c3d`.

### Rollback Mandate
If any of the following occur post-deployment:
1. Smoke test fails on `/health` or `/ready`.
2. Error rate exceeds 1% within 10 minutes of deployment.
3. P95 latency degrades past 3,000ms.

**Execute immediate rollback**:
- Using Azure CLI / OpenTofu: Shift 100% traffic back to the preceding stable Container App revision.
- Rollback target time: **< 60 seconds**.
- Rollback procedure is documented in `docs/rollback.md`.

---

## 5. Failure Drills & Chaos Scenarios

The system is tested against 4 intentional production failure scenarios:
1. **Scenario A (OCR Provider Outage)**: Gracefully return HTTP 504/502 with `ocr_failed`; do not crash or leak provider tokens.
2. **Scenario B (Database Outage)**: `/ready` immediately reports 503; traffic is drained.
3. **Scenario C (Broken Deployment)**: Failing smoke tests automatically block production promotion.
4. **Scenario D (Regression Drill)**: Simulated alert triggers rapid automated revision rollback.
