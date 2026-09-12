# ADR-006: Multi-Instance Rate Limiting Trade-offs (Distributed vs. Local Rate Limiting)

- **Status**: Accepted
- **Date**: 2026-09-12
- **Deciders**: Lensio Engineering Team
- **Technical Context**: `PRD Section 9` (Rate Limiting), `PRD Section 10` (Quota System), `PRD Section 22` (Azure Container Apps), `ADR-003` (Rate Limiting & Quota Architecture), `ADR-004` (ACA vs Kubernetes), `docs/benchmarks/load-test-report.md` (k6 RED Metrics Analysis)

---

## 1. Context and Problem Statement

In [ADR-003](./ADR-003-rate-limiting-and-quota-architecture.md), Lensio adopted a two-tier traffic-shaping architecture: an $O(1)$ in-memory token bucket limiter (`apps/api/internal/ratelimit`) on the HTTP hot path, paired with pre-OCR quota enforcement and asynchronous usage metering via buffered channels. In single-instance benchmarks, this in-memory implementation demonstrated sub-15-microsecond decision latency with zero database queries.

However, in accordance with [ADR-004](./ADR-004-azure-container-apps-vs-kubernetes.md), Lensio runs in production on **Azure Container Apps (ACA)** with KEDA HTTP autoscaling. As incoming API traffic grows, ACA dynamically scales container replicas from $N = 1$ to $N = 10$ behind an Envoy ingress and Cloudflare edge proxy.

This multi-replica topology introduces two critical architectural challenges:

### 1. The Multi-Replica Drift Phenomenon
In a distributed deployment without sticky sessions, incoming requests are load-balanced across $N$ running replicas (via round-robin or least-connections). Because each replica maintains an isolated, independent in-memory token bucket:
- **Maximum Aggregate Burst Drift**: An organization configured for a contractual limit of $L$ requests per minute (e.g., Starter: $30\,\text{rpm}$, Pro: $100\,\text{rpm}$) can theoretically execute up to:
  $$T_{\text{max}} = N \times L$$
  requests per minute across the cluster if traffic distributes uniformly across all $N$ replicas.
- **Skewed Premature Throttling**: Conversely, if upstream routing or client connection reuse sends a burst of requests to a single replica, that tenant could receive HTTP 429 (`rate_limit_exceeded`) even if neighboring replicas have idle token capacity.
- **Autoscaling Invalidation**: As KEDA scales replicas up or down, local bucket states are created or destroyed, leading to temporary counter resets.

### 2. Tail Latency Mutex Contention Under Peak Concurrency
During real 1,000 RPS k6 load testing (documented in [`docs/benchmarks/load-test-report.md`](../benchmarks/load-test-report.md)), we discovered that when hundreds of concurrent requests arrive for the *same tenant organization*, goroutines serialize on `b.mu.Lock()` inside `ratelimit.Limiter.Allow()`. While individual critical sections execute in $\approx 50\,\text{ns}$, thread contention on the single bucket mutex caused tail latency spikes up to $32\,\text{ms}$ at the 99th percentile (P99).

This ADR evaluates the trade-offs between local in-memory rate limiting, centralized distributed rate limiting (Redis), and edge-based rate limiting (Cloudflare), formalizes our decision, and establishes the architectural roadmap and Go interface contracts for distributed readiness.

---

## 2. Decision Drivers

1. **Upstream Vision AI Protection**: Strict rate limiting is non-negotiable to prevent exhausting upstream Google AI Studio rate limits and budget caps.
2. **Strict Hot-Path Latency Budget**: Rate limiting is evaluated on every HTTP request prior to multipart image decoding. The evaluation must add minimal overhead ($< 50\,\mu\text{s}$ for local vs. $< 2\,\text{ms}$ for networked systems).
3. **Ponytail Rule 1 (Minimal & Simplest Solution / YAGNI)**: Avoid introducing unnecessary infrastructure dependencies, maintenance costs, and failure modes before production scale demands them.
4. **Availability & Fail-Open Semantics**: The rate-limiting subsystem must never become a single point of failure (SPOF) that takes down the entire KTP OCR service.
5. **Standards Compliance**: Always return standard RFC headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`).

---

## 3. Considered Options

### Option 1: Status Quo — Local In-Memory Token Bucket with Sharded Partitioning

Each API replica maintains its own token buckets in RAM using Go standard library sync primitives (`sync.RWMutex`, `sync.Mutex`). To eliminate the `b.mu` mutex contention observed in load testing, bucket access is partitioned across a fixed stripe array (e.g., 32 bucket shards indexed by `hash(orgID) % 32`).

- **Mathematical Drift Model**:
  $$\text{Effective Cluster Limit} = \sum_{i=1}^{N} \text{Tokens}_i \le N \times L$$
  $$\text{Expected Request Share per Replica} = \frac{R}{N} \pm \sqrt{\frac{R(N-1)}{N^2}} \quad (\text{Poisson approximation})$$
  For $N = 2$ or $N = 3$ replicas, aggregate burst capacity is at most $2\times$ to $3\times$ the single-node limit during absolute peak uniform distribution.
- **Pros**:
  - **Blazing Speed**: Sub-microsecond execution ($< 10\,\mu\text{s}$) with zero network serialization or socket I/O.
  - **Zero Cost**: Zero external cloud infrastructure fees (Azure Cache for Redis costs $\$40\text{--}\$150+/\text{month}$).
  - **Maximum Reliability**: Zero external network dependencies; cannot fail due to network partitions or Redis outages.
  - **Stack Allocation Friendly**: $O(1)$ memory footprint ($\approx 200\,\text{bytes}$ per active tenant).
- **Cons**:
  - Cluster-wide rate enforcement is approximate rather than exact; limit scales linearly with active replica count $N$.
  - Ephemeral state: restarts or autoscaling scale-down events clear accumulated token history.

---

### Option 2: Distributed Centralized Redis Rate Limiter with Atomic Lua Scripts

A centralized Redis instance or Redis Cluster (e.g., Azure Cache for Redis Basic/Standard) shared by all Azure Container App replicas over the internal Azure Virtual Network (VNet). Rate evaluations execute via an atomic Redis Lua script implementing either a **sliding window counter** or an **atomic token bucket with server timestamp refill**.

#### Architecture:
```text
Client Request ──► [ACA Envoy] ──► [Lensio Replica k]
                                           │
                           (Redis EVALSHA over VNet, <2ms)
                                           ▼
                                 [Azure Cache for Redis]
                                 (Atomic Token Bucket Lua)
```

#### Redis Token Bucket Lua Script (`ratelimit.lua`):
```lua
-- KEYS[1]: rate_limit:{org_id}
-- ARGV[1]: capacity (float)
-- ARGV[2]: refill_rate_per_sec (float)
-- ARGV[3]: requested_tokens (float, usually 1)
-- ARGV[4]: current_time_sec (float)

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local requested = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if not tokens then
    tokens = capacity
    last_refill = now
else
    local elapsed = math.max(0, now - last_refill)
    tokens = math.min(capacity, tokens + (elapsed * refill_rate))
    last_refill = now
end

local allowed = 0
local remaining = 0
local retry_after = 0

if tokens >= requested then
    tokens = tokens - requested
    allowed = 1
    remaining = math.floor(tokens)
else
    local missing = requested - tokens
    retry_after = math.ceil(missing / refill_rate)
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
redis.call('EXPIRE', key, math.ceil(capacity / refill_rate) + 60)

return { allowed, remaining, retry_after }
```

- **Pros**:
  - **Exact Global Consistency**: 100% strict enforcement across $N$ container replicas with zero replica drift ($\pm 0\%$).
  - **Persistence Across Autoscaling**: New replicas immediately share the global rate-limit state; scaling from 1 to 10 instances does not loosen limits.
  - **Eliminates Local Mutex Contention**: Redis single-threaded event loop processes Lua commands serially, eliminating in-process Go mutex contention.
- **Cons**:
  - **Network Latency Penalty**: Adds a network round-trip ($0.8\text{--}2.5\,\text{ms}$) to *every single HTTP request*, increasing P50 and P95 latency.
  - **Infrastructure Cost**: Minimum $\$40\text{--}\$100/\text{month}$ for managed Azure Cache for Redis (Basic C0/Standard C1).
  - **New Failure Domain**: If Redis experiences latency degradation, failover, or connection exhaustion, API request processing can stall unless sophisticated circuit breakers and fallback modes are built.

---

### Option 3: Edge Rate Limiting via Cloudflare (WAF / API Shield)

Enforcing rate limits at Cloudflare's Anycast global edge network before requests ever traverse the public internet to Azure Container Apps.

- **Capabilities**: Cloudflare Rate Limiting rules match on request paths (e.g. `POST /api/v1/ocr/*`), HTTP headers (`Authorization` or `X-API-Key`), and client IP addresses.
- **Pros**:
  - **Zero Origin Load**: Throttled requests are rejected with 429 directly at the edge; zero CPU, memory, or bandwidth consumed on ACA.
  - **DDoS Immunity**: Edge absorbs massive volumetric floods (e.g., 50,000 RPS attacks) without origin degradation.
- **Cons**:
  - **Coarse Multi-Tenant Granularity**: Cloudflare edge rules cannot dynamically query PostgreSQL to inspect an organization's specific subscription tier (`free`: 10 rpm, `starter`: 30 rpm, `pro`: 100 rpm) without complex and costly Cloudflare Workers + Workers KV bindings.
  - **Billing Cost**: Cloudflare charges $\$0.15$ per 10,000 requests evaluated after the first 10,000 for standard rate-limiting rules.
  - **Vendor Lock-in**: Logic is tied to Cloudflare proprietary configuration rather than portable open-source code.

---

### Option 4: Multi-Tier Hybrid Strategy (Edge Coarse Shield + In-Memory Local + Distributed Redis Adapter)

A layered defense-in-depth model that combines the strengths of edge, local, and distributed tiers:
1. **Tier 1 (Edge - Cloudflare)**: Coarse volumetric DDoS rate limiting (e.g., 200 req/min per IP/API key) to protect Azure ingress from brute-force floods.
2. **Tier 2 (Gateway Middleware - Go `ratelimit.RateLimiter`)**: Pluggable interface in the Go API supporting both:
   - `Limiter` (In-memory token bucket, active default).
   - `RedisLimiter` (Distributed token bucket, opt-in for enterprise scale).
3. **Resilience Fallback**: If `RedisLimiter` is active and Redis becomes unreachable or times out ($> 30\,\text{ms}$), the adapter automatically **fails open** to local in-memory limiting and emits a Prometheus alert.

---

## 4. Decision Outcome

**Chosen Strategy**: **Phase-Gated Adoption of Option 4 (In-Memory Default + Pluggable `RateLimiter` Interface + Defined Redis Trigger)**

In strict adherence to **Ponytail Rule 1 (Minimal & Simplest Solution / YAGNI)**:
1. We will **NOT** prematurely provision Azure Cache for Redis for the current scale ($N \le 3$ replicas).
2. We **ACCEPT** bounded replica drift ($N \times L$) as an acceptable operational trade-off for early production, because:
   - Lensio's downstream cost is primarily gated by the **monthly quota check** ([ADR-003](./ADR-003-rate-limiting-and-quota-architecture.md)), which is evaluated against PostgreSQL and enforced independently of short-term rate bursts.
   - Even with $N = 2$ replicas, a Free-tier user bursting $20\,\text{rpm}$ instead of $10\,\text{rpm}$ still exhausts their $100\,\text{req/month}$ quota in 5 minutes, preventing financial bleed.
3. We decouple the API middleware by defining a clean Go interface contract (`ratelimit.RateLimiter`) in `apps/api/internal/ratelimit/ratelimit.go`, ensuring zero-downtime migration when the distributed engine is deployed.

```mermaid
flowchart TD
    Client[API Client / Consumer] -->|HTTPS Request| CF[Cloudflare Edge WAF]
    CF -->|Coarse DDoS Filter| ACA[Azure Container Apps Envoy Ingress]
    ACA -->|Load Balances Across Replicas| R1[Replica 1]
    ACA -->|Load Balances Across Replicas| R2[Replica 2]
    ACA -->|Load Balances Across Replicas| RN[Replica N]

    subgraph Replica Internal Logic
        R1 --> MW[RateLimit Middleware]
        MW --> RL{RateLimiter Interface}
        RL -->|Phase 1: Active Default| Mem[In-Memory Token Bucket\nO(1) Memory, <15µs]
        RL -.->|Phase 2: Enterprise Trigger| Redis[Distributed Redis Adapter\nAtomic Lua Script, <2ms]
    end

    Mem -->|Allowed| Quota[Monthly Quota Check\nPostgreSQL Indexed Query]
    Redis -->|Allowed| Quota
    Mem -->|Exceeded| E429[HTTP 429 rate_limit_exceeded]
    Redis -->|Exceeded| E429
    Quota -->|Allowed| OCR[OCR Processing Pipeline]
    Quota -->|Exceeded| EQ429[HTTP 429 quota_exceeded]
```

### Scaling Trigger Criteria (When to Deploy Redis)

We will activate the distributed Redis rate-limiting adapter when **any** of the following quantitative conditions is met:

| Metric / Trigger | Threshold | Justification |
|---|---|---|
| **Sustained Replica Count** | $N > 3$ replicas on ACA for $> 2\,\text{hours}$ | At $N = 4$, replica drift reaches $4\times$ ($40\,\text{rpm}$ on a $10\,\text{rpm}$ plan), which begins to meaningfully skew tier value. |
| **P99 Lock Contention** | In-memory limiter P99 latency $> 10\,\text{ms}$ under load | Indicates extreme tenant lock serialization that requires distributed offloading or partitioned hashing. |
| **Enterprise SLA Requirement** | Enterprise tenant with strict $\pm 0\%$ burst tolerance | Enterprise contracts specifying contractual burst capping across regions. |
| **Multi-Region Active-Active** | Deploying secondary ACA region (e.g. Southeast Asia + East Asia) | Cross-region traffic requires centralized state coordination. |

---

## 5. Distributed Redis Adapter Interface Design

To make the rate-limiting subsystem fully pluggable without violating YAGNI, we formalized the `RateLimiter` interface in `apps/api/internal/ratelimit/ratelimit.go`:

```go
package ratelimit

// Result contains the rate limiting evaluation decision and standard RFC header values.
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetTime  int64 // Unix timestamp (seconds)
	RetryAfter int   // Seconds to wait before retry (when Allowed is false)
}

// RateLimiter defines the pluggable contract for evaluating rate limits across local and distributed topologies.
type RateLimiter interface {
	Allow(key string, limitPerMinute int) Result
}
```

### Future Redis Adapter Specification (`apps/api/internal/ratelimit/redis.go`):

```go
// RedisLimiter implements RateLimiter using an external Redis instance with atomic Lua scripts.
type RedisLimiter struct {
	client       *redis.Client
	scriptSHA    string
	fallback     *Limiter        // Local in-memory fallback if Redis fails
	timeout      time.Duration   // Strict timeout (e.g., 25ms)
	circuitOpen  atomic.Bool
}

func (r *RedisLimiter) Allow(key string, limitPerMinute int) Result {
	if r.circuitOpen.Load() {
		return r.fallback.Allow(key, limitPerMinute)
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// Execute EVALSHA with key and arguments (capacity, refill rate, now)
	res, err := r.evalLua(ctx, key, limitPerMinute)
	if err != nil {
		// Fail-open: Record metric and delegate to in-memory fallback
		telemetry.RecordRateLimitRedisFallback(key)
		return r.fallback.Allow(key, limitPerMinute)
	}
	return res
}
```

### Resilience & Fail-Open Semantics
If the Redis cluster is unreachable, network-partitioned, or exceeds its $25\,\text{ms}$ timeout:
1. The adapter **fails open** to the local in-memory token bucket.
2. The failure increments the Prometheus counter `lensio_ratelimit_redis_fallback_total{reason="timeout|error"}`.
3. If consecutive Redis errors exceed 10 within 5 seconds, an internal circuit breaker trips to open state for 15 seconds, preventing Redis latency from cascading into client request timeouts.

---

## 6. Algorithmic Complexity & Big-O Benchmark Comparison

| Dimension | Local In-Memory Token Bucket (Current) | Distributed Redis Token Bucket (Future) | Cloudflare Edge Rate Limiting |
|---|---|---|---|
| **Time Complexity** | $O(1)$ | $O(1)$ (Redis hash lookup + math in Lua) | $O(1)$ (Edge hash map) |
| **Auxiliary Space** | $O(K)$ where $K$ is active tenant count ($\approx 200\,\text{B}/\text{tenant}$) | $O(K)$ stored in Redis RAM ($\approx 128\,\text{B}/\text{tenant}$) | Managed at edge |
| **Network Overhead** | **$0\,\text{ms}$** (In-process memory read/write) | **$0.8\text{--}2.5\,\text{ms}$** (TCP/TLS round-trip over VNet) | **$0\,\text{ms}$ added** (Evaluated in transit) |
| **P50 Latency Overhead** | $< 0.015\,\text{ms}$ ($15\,\mu\text{s}$) | $\approx 1.2\,\text{ms}$ | $0\,\text{ms}$ added to origin |
| **P99 Latency Overhead** | $\approx 0.050\,\text{ms}$ ($\approx 32\,\text{ms}$ under unpartitioned 1,000 RPS burst) | $\approx 4.5\,\text{ms}$ | $0\,\text{ms}$ added to origin |
| **Cluster Drift Across $N$ Replicas** | Up to $N \times L$ | **$\pm 0\%$** (Strictly synchronized) | Approximate across Anycast PoPs |
| **Infrastructure Cost** | **$\$0.00/\text{month}$** | $\$40\text{--}\$150+/\text{month}$ (Azure Cache for Redis) | $\$5\text{--}\$50+/\text{month}$ |
| **Failure Blast Radius** | Isolated to single container instance | Centralized SPOF if Redis fails without fallback | Edge outage impacts all ingress traffic |

---

## 7. Consequences

### Positive
- **Guaranteed High Performance**: Retaining the in-memory token bucket keeps the API's P50 hot path fast ($< 15\,\mu\text{s}$ rate limiting check), keeping overall API response times well below our $500\,\text{ms}$ SLO.
- **Architectural Agility**: By introducing the `ratelimit.RateLimiter` interface, the HTTP router and middleware are completely decoupled from storage implementation details.
- **Financial Prudence (Ponytail Rule 1)**: Avoids incurring $\$40\text{--}\$150/\text{month}$ in premature Azure managed Redis costs while the platform operates on $1\text{--}3$ ACA replicas.
- **Resilience Assurance**: The documented fallback strategy ensures that future distributed rate-limiting adoption will never cause widespread API outages.

### Negative / Trade-offs
- **Accepted Drift**: For $N = 2$ or $N = 3$ container replicas, an aggressive tenant can consume up to $2\times$ or $3\times$ their burst limit if requests distribute evenly across replicas. This is mitigated by our downstream monthly quota check in PostgreSQL.
- **Autoscaling Volatility**: When a container replica is terminated by KEDA, its local in-memory token counters are lost, granting a minor burst replenishment on new replicas.

---

## 8. References

- [`AGENTS.md`](../../AGENTS.md) - Rule 1 (Ponytail / Minimal Solution) & Rule 3 (Big-O Algorithmic Efficiency)
- [`docs/decisions/ADR-003-rate-limiting-and-quota-architecture.md`](./ADR-003-rate-limiting-and-quota-architecture.md) - Two-Tier Traffic Shaping Architecture
- [`docs/decisions/ADR-004-azure-container-apps-vs-kubernetes.md`](./ADR-004-azure-container-apps-vs-kubernetes.md) - ACA Serverless Deployment
- [`docs/benchmarks/load-test-report.md`](../benchmarks/load-test-report.md) - RED Metrics and Mutex Contention Findings
- [`apps/api/internal/ratelimit/ratelimit.go`](file:///home/amirfaisalz/Documents/amir/golang/lensio/apps/api/internal/ratelimit/ratelimit.go) - Go RateLimiter Interface & In-Memory Token Bucket
