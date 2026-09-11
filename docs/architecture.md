# Lensio System Architecture & Technical Design

> Production architecture specification for Lensio, an Indonesian KTP OCR API platform.  
> Details system components, C4 architectural diagrams, request lifecycles, and fault isolation domains.

---

## 1. System Overview & Core Purpose

Lensio is architected as an API-first B2B software platform. It ingests identity documents (specifically Indonesian KTP cards), classifies and validates document integrity, orchestrates multimodal vision AI inference (Google Gemini 2.0 Flash or deterministic mock engines), extracts structured identity fields (16-digit NIK, full legal name, date/place of birth, address, marital status, religion), and returns deterministic, validated JSON payloads.

Surrounding the core OCR engine is a complete platform engineering lifecycle:
- **Tenant Management & Authentication**: Bearer API keys with SHA-256 one-way hashing and scoped access control.
- **Traffic Protection**: In-memory token bucket rate limiting and monthly quota enforcement.
- **Asynchronous Telemetry**: Non-blocking usage metering, distributed tracing via OpenTelemetry, and Prometheus metrics.
- **Edge Security & Infrastructure**: Cloudflare WAF, Azure Container Apps with KEDA autoscaling, and private PostgreSQL database storage.

---

## 2. C4 Architecture Models

### Level 1: System Context Diagram

Visualizes how external developers, dashboard administrators, and downstream AI services interact with the Lensio boundary:

```mermaid
C4Context
    title System Context Diagram for Lensio Platform

    Person(developer, "API Consumer / Developer", "Integrates identity verification into applications (e.g. VeriForm, RentEase).")
    Person(admin, "Platform Admin", "Monitors API usage, rotates API keys, and tracks quotas via Developer Dashboard.")

    Enterprise_Boundary(b0, "Lensio Boundary") {
        System(lensio_api, "Lensio API Service", "Go REST API handling authentication, rate limiting, and KTP OCR pipeline.")
        System(lensio_dash, "Developer Dashboard", "React + TypeScript SPA for key management and telemetry charts.")
    }

    System_Ext(gemini, "Google AI Studio / Gemini Vision", "Multimodal LLM extracting raw text and JSON fields from images.")
    System_Ext(cloudflare, "Cloudflare Edge", "DNS, WAF, DDoS protection, and SSL termination.")
    System_Ext(grafana, "Observability Platform", "Prometheus + Grafana for RED metrics and distributed trace spans.")

    Rel(developer, cloudflare, "Submits KTP OCR requests (HTTPS)", "JSON / Multipart")
    Rel(admin, cloudflare, "Manages keys & views analytics", "HTTPS / Web")
    Rel(cloudflare, lensio_api, "Proxies API traffic to ACA", "HTTP / Bearer")
    Rel(cloudflare, lensio_dash, "Serves static assets", "HTTPS")
    Rel(lensio_api, gemini, "Sends image buffer for inference", "REST / TLS 1.3")
    Rel(lensio_api, grafana, "Pushes traces & Prometheus metrics", "OTLP / gRPC")
```

---

### Level 2: Container Diagram

Breaks down the operational containers running within the Azure Container Apps environment:

```mermaid
C4Container
    title Container Diagram for Lensio Platform

    Person(client, "Client Applications", "VeriForm, RentEase, or third-party mobile/web backends")

    Container_Boundary(aca, "Azure Container Apps (ACA)") {
        Container(api_gateway, "API Gateway & Engine", "Go 1.22+, Distroless Container", "Handles HTTP endpoints, rate limiting, OCR pipeline, and usage dispatch.")
        Container(dashboard_app, "Dashboard Frontend", "Nginx + React 18, Alpine", "Static SPA providing developer portal UI.")
    }

    ContainerDb(postgres, "PostgreSQL 16", "Azure Flexible Server", "Stores organizations, hashed API keys, plans, audit logs, and usage records.")
    System_Ext(vision_engine, "Vision Provider", "Gemini 2.0 Flash / Mock", "Extracts KTP structured text.")

    Rel(client, api_gateway, "POST /api/v1/ocr/ktp", "HTTPS / Multipart")
    Rel(client, dashboard_app, "Navigates portal", "HTTPS")
    Rel(api_gateway, postgres, "Queries keys, records usage", "pgx / TCP 5432")
    Rel(api_gateway, vision_engine, "Extracts fields via OCREngine", "HTTP")
```

---

### Level 3: Component Diagram (Go API Service)

Details the internal Go modular layers within `apps/api/`:

```mermaid
C4Component
    title Component Diagram for apps/api (Go Service)

    Component(router, "ServeMux Router", "net/http", "Directs incoming requests to middleware chains and handlers.")
    Component(mw_trace, "Telemetry Middleware", "telemetry", "Injects RequestID, OTel trace context, and logs request start/stop.")
    Component(mw_auth, "Auth & Scope Middleware", "middleware", "Validates SHA-256 API key hash and asserts required scopes.")
    Component(mw_rl, "Rate Limiter", "ratelimit", "Enforces in-memory token bucket per organization.")
    Component(mw_usage, "Usage Metering Dispatcher", "usage", "Captures response latency & dispatches usage event asynchronously.")

    Component(h_ocr, "KTP OCR Handler", "handlers", "Decodes multipart form, enforces 5MB limit, and validates MIME magic bytes.")
    Component(svc_quota, "Quota Enforcer", "quota", "Evaluates monthly usage against plan limits before inference.")
    Component(svc_ocr, "OCR Pipeline Service", "services/ocr", "Coordinates image normalization, OCREngine call, and field validation.")
    Component(val_nik, "Deterministic Validator", "ocr", "Strict 16-digit NIK structure, province code, and DOB validation.")

    Rel(router, mw_trace, "Applies")
    Rel(mw_trace, mw_auth, "Applies")
    Rel(mw_auth, mw_rl, "Applies")
    Rel(mw_rl, mw_usage, "Wraps")
    Rel(mw_usage, h_ocr, "Executes")
    Rel(h_ocr, svc_quota, "Checks monthly allowance")
    Rel(h_ocr, svc_ocr, "Invokes extraction")
    Rel(svc_ocr, val_nik, "Normalizes & validates output")
```

---

## 3. End-to-End Request Lifecycle & Sequence Flow

The following sequence diagram tracks a request through the full pipeline:

```mermaid
sequenceDiagram
    autonumber
    actor Client as API Client (VeriForm)
    participant Edge as Cloudflare Edge
    participant GW as Go API Router
    participant Auth as Auth & Rate Limiter
    participant Quota as Quota Enforcer
    participant OCR as OCR Engine (Gemini / Mock)
    participant Val as Deterministic Validator
    participant Meter as Async Usage Channel
    participant DB as PostgreSQL

    Client->>Edge: POST /api/v1/ocr/ktp (Bearer lensio_live_xxx, document=<image>)
    Edge->>GW: Forward request with X-Request-ID
    GW->>Auth: Compute SHA-256(token) & Check Token Bucket
    Auth->>DB: SELECT api_key WHERE hash=$1 (Cached/Indexed)
    DB-->>Auth: Key Valid (Scopes: ocr:write)
    Auth->>Quota: Check Monthly Quota Allowance
    Quota->>DB: Verify current monthly usage < plan.monthly_quota
    DB-->>Quota: Quota OK (Usage: 45 / 1,000)
    GW->>OCR: Validate magic bytes & Stream image []byte
    OCR-->>GW: Raw Extraction (NIK, Nama, DOB, etc.)
    GW->>Val: Validate NIK (16-digits, province code) & Date sanity
    Val-->>GW: Cleaned Structured Data + Confidence Score (0.98)
    GW-->>Client: HTTP 200 OK (Structured JSON response)
    Note over Client,GW: Latency: ~1,420ms (P95 SLA < 2,000ms)
    GW-)Meter: Push UsageRecord to Buffered Channel (Non-blocking)
    Meter-)DB: Background Worker writes INSERT INTO usage_records
```

---

## 4. Fault Isolation & Health Check Architecture

Lensio enforces strict decoupling between **Process Liveness** and **Dependency Readiness**:

```text
               Kubernetes / Azure Container Apps Probes
                    │                            │
                    ▼                            ▼
            GET /health (Liveness)        GET /ready (Readiness)
                    │                            │
        ┌───────────┴───────────┐        ┌───────┴───────┐
        │ Returns HTTP 200 OK   │        │ Pings DB Pool │
        │ if Go process is alive│        └───────┬───────┘
        └───────────────────────┘                │
                                      ┌──────────┴──────────┐
                                      │                     │
                                Connected (200 OK)    Down / Unreachable (503)
                                      │                     │
                                Routes Traffic        Halts Traffic Routing
                                                      (Container Not Killed)
```

1. **Liveness Probe (`GET /health`)**:
   - Tests only the internal state of the Go process and HTTP listener.
   - Never calls PostgreSQL or upstream APIs.
   - If this fails, the container is deadlocked and ACA restarts the container.
2. **Readiness Probe (`GET /ready`)**:
   - Tests PostgreSQL connection pool using `db.PingContext(ctx)`.
   - Returns HTTP 200 `{"status":"ready","database":"connected"}` when healthy.
   - Returns HTTP 503 `{"status":"not_ready","database":"unreachable"}` when the database is unavailable.
   - Prevents traffic routing to instances that cannot serve queries without causing destructive crash-restart loops.

---

## 5. Security & Privacy Boundaries

1. **Data Minimization Boundary**: The `document` image buffer exists exclusively in volatile RAM during the HTTP handler execution. It is never persisted to disk, blob storage, or database.
2. **Zero PII Logging Boundary**: The structured logger (`log/slog`) rejects all citizen identity fields. Only `request_id`, `trace_id`, status code, latency, and confidence score are written to output streams.
3. **Secret Isolation**: Secrets (DB passwords, AI Studio API keys) are injected exclusively via environment variables from Azure Key Vault or `.env` in local development.

---

## 6. Technology Inventory

| Component | Technology | Role |
|---|---|---|
| **API Runtime** | Go 1.22+ | HTTP routing, concurrency, business logic, validation |
| **Relational Database** | PostgreSQL 16 | Tenant data, API keys, plans, usage metrics, audit trail |
| **Database Driver** | `jackc/pgx/v5` | High-performance Go native PostgreSQL connection pool |
| **Vision AI Provider** | Google Gemini 2.0 Flash | Upstream multimodal image extraction |
| **Test Vision Engine** | `MockOCREngine` | Deterministic synthetic fixture engine for CI |
| **Developer Dashboard** | React 18 + Vite + TypeScript | Frontend developer portal and usage analytics |
| **Telemetry & Metrics** | OpenTelemetry Go SDK + Prometheus | Distributed tracing, RED metrics |
| **Cloud Hosting** | Azure Container Apps (ACA) | Serverless container execution with KEDA autoscaling |
| **Edge & Security** | Cloudflare | Edge CDN, WAF, DDoS protection, TLS 1.3 termination |
| **IaC Orchestration** | OpenTofu + Terragrunt | Declarative multi-environment infrastructure as code |
