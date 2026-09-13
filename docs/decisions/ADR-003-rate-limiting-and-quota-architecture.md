# ADR-003: Rate Limiting and Quota Architecture

- **Status**: Accepted
- **Date**: 2026-09-12
- **Deciders**: Lensio Engineering Team
- **Technical Context**: `PRD Section 9`, `PRD Section 10`, `PRD Section 11`, `PRD Section 30`

---

## 1. Context and Problem Statement

As a commercial API product, Lensio must protect its upstream AI vision providers (e.g. Gemini 2.0 Flash) and infrastructure from abusive bursts, accidental infinite loops, and distributed denial-of-service (DoS) attacks. Furthermore, the platform offers tiered subscription plans (`free`, `starter`, `pro`) with contractual limits on:
1. **Burst Concurrency / Rate Limits**: Short-term requests per minute (e.g., Free: 10 rpm, Starter: 30 rpm, Pro: 100 rpm).
2. **Billing Volume / Monthly Quotas**: Long-term requests per calendar month (e.g., Free: 100/mo, Starter: 1,000/mo, Pro: 10,000/mo).

Executing synchronous database writes or transactions on every incoming HTTP request to check or increment counters would introduce serious database locking, connection pool contention, and significant latency penalties (>50–100ms) on hot paths. We needed an architecture that enforces strict rate limits and quotas while maintaining low API latency.

---

## 2. Decision Drivers

- **Sub-Millisecond Overhead**: Rate limit checks must execute in microseconds ($O(1)$) at the transport/middleware layer before any multipart image decoding occurs.
- **Upstream Protection**: Prevent exhaustion of upstream Google AI Studio rate limits and budget caps.
- **Standards Compliance**: Return standard IETF/RFC rate-limiting headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`).
- **Semantic Error Differentiation**: Differentiate between short-term rate throttling (`rate_limit_exceeded`) and monthly billing exhaustion (`quota_exceeded`) using distinct error codes and HTTP 429 responses.
- **Zero Lock Contention on Usage Writes**: Recording usage analytics must never block the client's HTTP response.

---

## 3. Considered Options

### Option 1: Synchronous PostgreSQL Transactions (`SELECT FOR UPDATE`)
- *Pros*: Strict ACID consistency across multiple horizontal API instances without additional infrastructure.
- *Cons*: Catastrophic performance bottleneck. Every concurrent request acquires row-level locks on the tenant record, causing lock serialization, connection starvation, and latency spikes under load.

### Option 2: Distributed Redis Cluster
- *Pros*: Centralized token bucket counters across all container replicas.
- *Cons*: Violates Ponytail Rule 1 (Minimal & Simplest Solution / YAGNI for current MVP scale). Introduces another infrastructure component, VPC peering overhead, additional latency network hop per request, and higher minimum infrastructure costs on Azure.

### Option 3: Two-Tier In-Memory Rate Limiting + Async Metered Quota Evaluation (Selected)
- *Pros*:
  - **Tier 1 (In-Memory Token Bucket / Sliding Window)**: Each API gateway instance maintains thread-safe in-memory rate limiters keyed by Organization ID or API Key. Checks execute in $O(1)$ memory time (<10 microseconds) with zero database queries.
  - **Tier 2 (Pre-Inference Quota Check)**: Monthly quota usage is evaluated prior to executing costly OCR inference using lightweight indexed aggregate queries or cached counters.
  - **Asynchronous Metering Engine**: Usage records (`usage_records`) are pushed onto a buffered Go channel (`chan *UsageRecord`) and written in background worker goroutines. The client receives their HTTP response immediately upon OCR completion without waiting for analytics persistence.
- *Cons*: In multi-replica deployments without sticky routing, in-memory rate limits are enforced per-instance (e.g., 2 instances with 10 rpm allow up to 20 rpm aggregate). This is acceptable for MVP and can easily transition to Redis when traffic scale dictates.

---

## 4. Decision Outcome

**Chosen Option**: **Option 3 (Two-Tier Architecture with Non-Blocking Asynchronous Metering)**

### Architecture Breakdown:

```text
HTTP Request
     │
     ▼
[RateLimit Middleware]  ──(Exceeded?)──► HTTP 429 "rate_limit_exceeded"
     │ (In-memory O(1))                   Headers: X-RateLimit-*, Retry-After
     ▼
[Quota Enforcer]        ──(Exceeded?)──► HTTP 429 "quota_exceeded"
     │ (Pre-OCR Check)                    Headers: X-RateLimit-*
     ▼
[OCR Pipeline Handler]
     │ (Gemini Flash / Mock OCR)
     ▼
[HTTP Response: 200 OK] (Sent to client immediately)
     │
     ▼
[Buffered Usage Channel] (chan store.UsageRecord, capacity: 1,024)
     │
     ▼ (Asynchronous Background Worker)
[PostgreSQL usage_records]
```

### Plan Tier Configuration Matrix:

| Plan Code | Monthly Quota | Rate Limit (req/min) | Max Burst | Over-Quota Policy |
|---|---|---|---|---|
| `free` | 100 requests | 10 req/min | 10 | Strict 429 block |
| `starter` | 1,000 requests | 30 req/min | 30 | Strict 429 block |
| `pro` | 10,000 requests | 100 req/min | 100 | Strict 429 block |

---

## 5. Consequences

### Positive
- **Near-Zero Latency Overhead**: Rate limit evaluations add less than 15µs to the request hot path.
- **Resilient to Spikes**: If database latency temporarily increases, incoming requests continue to be served smoothly because usage persistence is decoupled via buffered channels.
- **Clear Developer Feedback**: Clients know exactly when their rate limit will reset via standard headers and can implement automated exponential backoff with `Retry-After`.

### Negative / Trade-offs
- Graceful shutdown logic (`server.Shutdown`) must drain the buffered usage channel to ensure no usage records are lost during container restarts or scale-downs (implemented in `Recorder.Close()`).

---

## 6. References
- `AGENTS.md` - Rule 1 (Ponytail Architecture) & Rule 3 (Algorithmic Efficiency)
- `PRD Section 9` - Rate Limiting
- `PRD Section 10` - Quota System
- `PRD Section 11` - Usage Metering

## 7. Amendment 2026-09-13: No-Bypass Per-Identity Buckets
Historical bypasses for OIDC/dashboard sessions and unresolved orgs were removed (`apps/api/internal/http/middleware/ratelimit.go`). The bucket key is now `API-key org → oidc:sub → default org → ip → anonymous`; non-org keys (`oidc:`, `ip:`, `anonymous`) resolve to the default free-tier limit without a plan lookup. Rationale: an unlimited dashboard/OIDC path is a DoS vector. Prior behavior is preserved in git history.
