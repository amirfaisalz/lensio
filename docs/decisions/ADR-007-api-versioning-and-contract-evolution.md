# ADR-007: API Versioning Strategy and Contract Evolution

- **Status**: Accepted
- **Date**: 2026-09-12
- **Deciders**: Lensio Engineering Team
- **Technical Context**: `PRD Section 6` (API Versioning), `PRD Section 29` (OpenAPI Contract), `PRD Section 30` (Client SDK & Ergonomics), `ADR-001` (Go for API), `ADR-002` (API Keys & Security), `ADR-003` (Rate Limiting & Quotas), `ADR-006` (Multi-Instance Trade-offs)

---

## 1. Context and Problem Statement

An HTTP API is not merely a backend implementation detail; it is a **legally and technically binding contract** published to external third-party software systems. In Lensio's production ecosystem, clients include fintech platforms, digital banks, peer-to-peer lenders, telecommunication providers, and identity verification portals across Indonesia. These client systems compile or bind against Lensio's schemas to automate mission-critical customer onboarding (KYC).

If Lensio introduces unannounced or unexpected contract modifications:
- Client production deployments fail silently or throw unhandled deserialization errors.
- End-user onboarding halts, generating direct financial and reputational losses for our business customers.
- Trust in the Lensio platform degrades, causing customer churn and SLA disputes.

We must define a deterministic, production-grade strategy for **API versioning**, establish unambiguous definitions for **breaking vs. non-breaking changes**, specify the **contract deprecation and sunset lifecycle**, and codify **safe schema evolution patterns** for request and response payloads.

---

## 2. Decision Drivers

1. **Client Empathy & Zero Surprise Breakages**: Once an API endpoint is marked production-ready (GA), internal architectural refactors or engine optimizations must never break external integrations.
2. **Predictable Upgrade Cycles**: Customers must receive ample advance notice, standardized machine-readable headers, and unambiguous migration documentation prior to any decommission.
3. **Standards Adherence**: Utilize recognized IETF standards and RFCs for deprecation signals:
   - **RFC 8594**: The `Sunset` HTTP Response Header.
   - **IETF RFC Draft (draft-ietf-httpapi-deprecation-header)**: The `Deprecation` HTTP Response Header.
   - **RFC 8288**: Web Linking via the `Link` Header for successor and migration references.
4. **Postel's Law (Robustness Principle)**: *"Be conservative in what you send, be liberal in what you accept."*
5. **Operational Simplicity (Ponytail Rule 1)**: The versioning mechanism must be trivial to parse, cache, route at the edge (Cloudflare / Azure Container Apps Envoy ingress), and document in OpenAPI 3.0 schemas.

---

## 3. Considered Versioning Strategies

We evaluated four industry patterns for API contract versioning:

### Option 1: URL Path Versioning (`/api/v1/...`, `/api/v2/...`) [Selected]

Major versions are encoded directly in the URL path prefix (e.g., `POST /api/v1/ocr/ktp` vs. `POST /api/v2/ocr/ktp`).

- **Pros**:
  - **Unambiguous Routing**: Reverse proxies, edge gateways (Cloudflare), and ACA Envoy ingresses can route traffic to different backend container revisions or handlers based solely on path matching without inspecting headers or bodies.
  - **Transparent Observability**: Metrics, distributed traces, and access logs immediately differentiate version usage in Grafana/Prometheus (e.g., `http_requests_total{path=~"/api/v1/.*"}`).
  - **Tooling Ergonomics**: Standard OpenAPI specifications, Swagger UI, Postman collections, and cURL commands work seamlessly without custom header orchestration.
  - **Zero Ambiguity for Developers**: Third-party developers cannot inadvertently default to the wrong version due to missing request headers.
- **Cons**:
  - URL technically identifies a resource rather than a representation; path versioning couples resource identifiers to schema revisions.

---

### Option 2: Header-Based Versioning (`Accept` or Custom Header)

Version is passed via HTTP headers, e.g., `Accept: application/vnd.lensio.v1+json` or `X-API-Version: 1.0`.

- **Pros**: Clean RESTful URIs that represent pure resources (`POST /api/ocr/ktp`).
- **Cons**:
  - Requires inspection of request headers before routing at the edge, complicating Cloudflare edge caching and ACA ingress routing rules.
  - Developer friction: developers frequently forget headers in quick cURL scripts or webhook tests, leading to confusion when default fallback behavior triggers.
  - Complicates OpenAPI schema generation and static documentation browsing.

---

### Option 3: Query Parameter Versioning (`/api/ocr/ktp?version=1`)

Version is supplied as an explicit query string parameter.

- **Pros**: Easy to test in a browser.
- **Cons**:
  - Query parameters are semantically intended for resource filtering, sorting, or pagination, not contract definition.
  - Vulnerable to accidental stripping by intermediate caching proxies or query sanitizers.
  - Clutters endpoint signatures.

---

### Option 4: Date-Based Request Header Versioning (Stripe Pattern)

Clients configure a fixed account-level version date (e.g., `Lensio-Version: 2026-09-12`), and the backend executes internal bidirectional request/response transformation gates.

- **Pros**: Maximum continuous evolution with zero URL changes; clients upgrade at their own pace without changing endpoints.
- **Cons**:
  - Massive architectural complexity: requires maintaining dozens of transformation adapters and historical snapshots in the gateway codebase.
  - High testing burden and combinatorial state space explosion.
  - Violates Ponytail Rule 1 (extreme over-engineering for an MVP/V1 identity service).

---

## 4. Decision Outcome

**Chosen Strategy**: **Option 1 — URL Path Versioning (`/api/v{N}/...`)**

Lensio standardizes on explicit URL path prefixes for major contract versions:
- Current Production Active: `/api/v1/ocr/ktp`
- Future Major Milestones: `/api/v2/ocr/ktp`

Minor and patch evolutions occur **in-place** within the active major version path using strictly **non-breaking, additive changes**.

---

## 5. Strict Taxonomy: Breaking vs. Non-Breaking Changes

To remove ambiguity during code reviews and architectural audits, Lensio enforces the following rigid categorization:

| Change Category | Description | Classification | Action Required |
|---|---|---|---|
| **Add Optional Request Parameter** | Adding a new optional field to a request body or query parameter | **Non-Breaking** | Allowed in `/api/v1/` with default fallback |
| **Add Response Field** | Adding a new field to a JSON response object (e.g., adding `rt_rw` or `confidence_score`) | **Non-Breaking** | Allowed in `/api/v1/` (Clients must practice tolerant reading) |
| **Add New Endpoint** | Introducing a new route (e.g., `POST /api/v1/ocr/sim` or `GET /api/v1/account/billing`) | **Non-Breaking** | Allowed in `/api/v1/` |
| **Relax Input Validation** | Accepting additional valid formats (e.g. supporting WebP images in addition to JPEG/PNG) | **Non-Breaking** | Allowed in `/api/v1/` |
| **Rename Request Field** | Renaming an existing input parameter (e.g. `image` $\rightarrow$ `ktp_file`) | **Breaking** | Prohibited in v1; Requires `/api/v2/` or dual-field transition |
| **Remove Request Field** | Removing support for an existing input parameter | **Breaking** | Prohibited in v1; Requires major version bump |
| **Make Optional Field Required** | Changing a field constraint from optional to required | **Breaking** | Prohibited in v1; Requires `/api/v2/` |
| **Rename/Remove Response Field** | Renaming or deleting an existing key from the JSON payload (e.g. renaming `nik` $\rightarrow$ `identity_number`) | **Breaking** | Prohibited in v1; Requires `/api/v2/` |
| **Change Data Type of Field** | Changing a string to an integer, or a single object to an array | **Breaking** | Prohibited in v1; Requires `/api/v2/` |
| **Alter Error Envelope Structure** | Changing `{ "error": { "code": "...", "message": "..." } }` structure | **Breaking** | Prohibited in v1; Strict consistency mandate |
| **Change HTTP Status Code for Existing Outcome** | E.g., returning 400 instead of 422 for malformed KTP image | **Breaking** | Prohibited in v1; Existing client error handlers will fail |
| **Tighten Input Validation** | Rejecting previously accepted formats or reducing max file size below 5MB | **Breaking** | Prohibited in v1; Requires `/api/v2/` |

### Client Compatibility Contract (Tolerant Reader Mandate)
All official Lensio SDKs and documentation instruct API consumers to implement the **Tolerant Reader Pattern**:
1. Clients **must ignore unknown JSON keys** when deserializing API response payloads.
2. Lensio reserves the right to add new fields to response payloads at any time without a major version increment.
3. Deserializers must not enable strict "disallow unknown fields" flags in client environments.

---

## 6. Contract Deprecation and Sunset Lifecycle

When a major API version is superseded or an endpoint is marked for deprecation, Lensio follows an explicit, machine-readable 4-phase lifecycle:

```text
┌────────────────┐      Successor Released       ┌────────────────────┐
│                │ ────────────────────────────► │                    │
│     ACTIVE     │                               │     DEPRECATED     │
│ (Default / GA) │ ◄──────────────────────────── │ (Emits Headers)    │
└────────────────┘       Emergency Rollback      └────────────────────┘
                                                           │
                                                           │ Sunset Date Reached
                                                           │ (Minimum 12 Months)
                                                           ▼
┌────────────────┐       Route Tear-down         ┌────────────────────┐
│                │ ◄──────────────────────────── │                    │
│    RETIRED     │                               │       SUNSET       │
│ (HTTP 404/Gone)│                               │   (HTTP 410 Gone)  │
└────────────────┘                               └────────────────────┘
```

### Phase Details and Timeline:

1. **Active (GA)**:
   - Endpoint receives full support, bug fixes, performance optimizations, and security patches.
   - Standard HTTP headers only.
2. **Deprecated (Notice Window: Minimum 12 Months)**:
   - A successor endpoint is released and generally available (e.g. `/api/v2/ocr/ktp`).
   - The deprecated endpoint continues to function with 100% fidelity and identical latency SLA.
   - Every response from the deprecated endpoint MUST include the standardized HTTP headers:
     ```http
     Deprecation: true
     Sunset: 2027-09-12
     Link: <https://docs.lensio.dev/migration/v1-to-v2>; rel="sunset"; type="text/html", <https://api.lensio.dev/api/v2/ocr/ktp>; rel="successor-version"
     ```
   - Telemetry tracks usage of deprecated endpoints per `org_id`.
   - Automated notification emails are dispatched at 12 months, 6 months, 3 months, 30 days, and 7 days prior to the Sunset date.
3. **Sunset (Terminal Grace Period: 30 Days)**:
   - The sunset date has arrived.
   - The endpoint returns **HTTP 410 Gone** with an actionable RFC 7807 error envelope:
     ```json
     {
       "error": {
         "code": "endpoint_sunset",
         "message": "The /api/v1/ocr/ktp endpoint was sunset on 2027-09-12. Please migrate to /api/v2/ocr/ktp.",
         "details": {
           "sunset_date": "2027-09-12",
           "migration_guide": "https://docs.lensio.dev/migration/v1-to-v2",
           "replacement": "/api/v2/ocr/ktp"
         }
       }
     }
     ```
   - For critical enterprise customers, brownout drills (intentionally serving 410 Gone for 2 hours during off-peak windows) are conducted 60 days before permanent sunset.
4. **Retired (Decommissioned)**:
   - Route is removed from the HTTP multiplexer.
   - Associated backend handlers, database columns, and adapter layers are safely purged from the codebase.

---

## 7. Technical Implementation: Go Deprecation Middleware

In Lensio's Go codebase (`apps/api/internal/http/middleware/deprecation.go`), deprecation signals are applied cleanly at the router level without contaminating business logic handlers:

```go
// DeprecationConfig configures deprecation, sunset, and linkage headers.
type DeprecationConfig struct {
    Deprecated bool
    Sunset     string // e.g. "2027-09-12" (ISO 8601) or RFC 1123 format
    Link       string // RFC 8288 link header value or migration guide URL
}

// DeprecationWithConfig wraps an http.Handler with standards-compliant deprecation metadata.
func DeprecationWithConfig(cfg DeprecationConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if cfg.Deprecated {
                w.Header().Set("Deprecation", "true")
                if cfg.Sunset != "" {
                    w.Header().Set("Sunset", cfg.Sunset)
                }
                if cfg.Link != "" {
                    w.Header().Set("Link", cfg.Link)
                }
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

This ensures that deprecating an endpoint requires exactly one declarative line in `apps/api/internal/http/router.go`:
```go
deprecatedMw := middleware.DeprecationWithConfig(middleware.DeprecationConfig{
    Deprecated: true,
    Sunset:     "2027-09-12",
    Link:       `<https://docs.lensio.dev/migration/v1-to-v2>; rel="sunset"`,
})
mux.Handle("POST /api/v1/ocr/ktp", authMiddleware(deprecatedMw(ktpHandler)))
```

---

## 8. Safe Evolution Patterns for Request & Response Payloads

When evolving schemas without bumping the major version, developers must employ the following safe evolution patterns:

### 1. Dual-Field Transition Pattern (For Field Replacements)
If a field name must be updated (e.g. transitioning from `nama` to `full_name`):
1. **Phase A (Additive)**: Add `full_name` to the response struct. Continue populating both `nama` and `full_name` with identical string values.
2. **Phase B (Doc Deprecation)**: Mark `nama` as deprecated in `openapi.yaml`. Document that `nama` will be removed in v2.
3. **Phase C (Major Bump)**: In `/api/v2/`, remove `nama` entirely.

### 2. Additive Enum Expansion
When introducing new enum values to response properties (e.g., adding `status: "manual_review_required"` to OCR extraction results):
- Document in the API contract that clients must include a default/fallback branch in enum parsing logic.
- Never recycle or redefine the semantic meaning of an existing enum string.

### 3. Automated Contract Testing & OpenAPI Diffing in CI
To guarantee that breaking changes are never inadvertently committed to `main`:
- CI pipelines execute automated schema comparisons (e.g., using `oasdiff` or Spectral).
- If a pull request modifies an existing path or schema in `openapi/openapi.yaml` in a backwards-incompatible manner without bumping the URL path prefix, the CI check automatically fails with:
  ```text
  [FAIL] Detected breaking change in /api/v1/ocr/ktp:
         Removed response property: 'data.nik'
         Breaking changes are strictly prohibited on /api/v1 routes.
  ```

---

## 9. Consequences

### Positive Consequences
- **High Enterprise Trust**: B2B enterprise clients can integrate Lensio into their core banking and KYC workflows with confidence that updates will never cause unannounced breaking outages.
- **Zero Ambiguity in Edge Routing**: ACA Envoy and Cloudflare can partition, rate-limit, and route traffic deterministically by URL prefix.
- **Decoupled Engine Upgrades**: The internal `OCREngine` (e.g., upgrading Gemini 1.5 Flash to Gemini 2.0 Flash) can evolve independently as long as the external `/api/v1/` output matches the deterministic schema.
- **Standards Compliance**: Clients consuming modern HTTP tooling automatically detect `Deprecation` and `Sunset` headers.

### Negative Consequences / Overheads
- **Maintenance Overlap**: Supporting both `/api/v1/` and `/api/v2/` simultaneously requires maintaining dual routes and regression suites for at least 12 months.
  - *Mitigation*: The v1 handler can wrap or map to the v2 internal service layer, acting as a lightweight compatibility adapter.
- **Client Inertia**: Some clients ignore deprecation headers and only act when their integration breaks.
  - *Mitigation*: Proactive out-of-band email notifications and targeted off-peak brownout drills prior to final sunset.

---

## 10. Interview Narrative

> *"An API is a published contract with third-party code. We never break client integrations on internal changes; contract evolutions follow our documented deprecation and sunset policy."*
