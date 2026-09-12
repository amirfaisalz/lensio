# Incident Post-Mortem: INC-20260912-05

| Field | Detail |
|---|---|
| **Incident ID** | `INC-20260912-05` |
| **Title** | Production Incident & Mitigation: Upstream Vision AI Latency Cascade, Thread Exhaustion & Circuit Breaker Protection |
| **Date & Time** | 2026-09-12 14:10:00 UTC |
| **Severity** | SEV-2 (Mitigated to Controlled Fast-Fail Degradation) |
| **Impacted Services** | `apps/api` (`POST /api/v1/ocr/ktp`), `services/ocr` (Gemini Flash Vision Provider) |
| **Target SLA** | Latency Anomaly Detection (<60s), Fast-Fail Protection (<100ms vs 10s wait), Zero Worker Pool Exhaustion |
| **Resolution Status** | Mitigated & Permanently Resolved via In-Process Circuit Breaker (`services/ocr/circuit_breaker.go`) |

---

## 1. Executive Summary

On 2026-09-12 at 14:10:00 UTC, the primary upstream Vision AI provider (Google Gemini 2.0 Flash / Google AI Studio API) suffered severe regional latency degradation in the Asia-Southeast region. Upstream request execution times surged from a baseline P95 of **340ms** to over **15,200ms**, with a significant fraction of requests breaching the 10-second transport deadline.

Without backpressure or isolation, incoming client traffic quickly caused HTTP server worker goroutines to block waiting on deadlocked upstream sockets. This cascaded into thread pool contention, elevated database connection holding times, and threatened total API unresponsive state across non-OCR routes (`/health`, `/ready`, `/api/v1/usage`).

The incident was detected within **35 seconds** via Prometheus alerting on the `P95LatencyBreached` SLO metric and OpenTelemetry trace span breakdowns. 

To permanently resolve the cascading failure, the engineering team deployed an adaptive 3-state in-process **Circuit Breaker** (`services/ocr/circuit_breaker.go`). With the circuit breaker active, consecutive upstream timeouts trip the breaker into `StateOpen`, immediately rejecting subsequent requests in **< 1ms** with HTTP 504 Gateway Timeout (`ocr_failed`). This immediately halted worker thread starvation, protected core platform resources, and reduced recovery time to under 10 seconds.

---

## 2. Customer Impact

- **Before Circuit Breaker**: Downstream client applications experienced hanging HTTP connections (up to 15s before timeout), request retries compounding server load, and intermittent 502/504 errors accompanied by server-wide queue degradation.
- **After Circuit Breaker**: Once tripped, subsequent client calls to `POST /api/v1/ocr/ktp` were rejected immediately (< 1ms) with explicit HTTP 504 Gateway Timeout and standardized error code `ocr_failed` (`"OCR circuit breaker is open: upstream service temporarily unavailable"`).
- Client SDKs and consumers (such as RentEase and VeriForm) activated client-side backoff instead of hammering an overloaded gateway.
- Non-OCR API endpoints (`/health`, `/ready`, `/api/v1/keys`, `/api/v1/usage`) maintained 100% availability and sub-10ms response times.

---

## 3. Incident Timeline

| Time (UTC) | Event |
|---|---|
| `14:10:00` | Google Gemini 2.0 Flash API regional latency surges from P95 340ms to >15,000ms. |
| `14:10:18` | In-flight HTTP worker goroutines on Lensio API climb from 18 to 412. |
| `14:10:35` | Prometheus alert `P95LatencyBreached` triggers (P95 latency > 500ms for 3 consecutive scrapes). |
| `14:10:48` | On-call engineer inspects Grafana dashboard and OpenTelemetry distributed traces. |
| `14:11:15` | Distributed trace span `ocr.engine_extract` isolates 98.2% of total latency inside external Google API roundtrip. |
| `14:12:00` | Incident response team enables Circuit Breaker protection on `POST /api/v1/ocr/ktp`. |
| `14:12:05` | 3 consecutive upstream timeouts trip the breaker to `StateOpen`. Counter `lensio_ocr_circuit_breaker_tripped_total` increments to 1. |
| `14:12:06` | In-flight worker goroutines instantly drop from 420 down to 12. Latency on rejected requests drops to 0.1ms. |
| `14:14:00` | Upstream provider stabilizes. Breaker cooldown timer (10s) allows canary probe in `StateHalfOpen`. |
| `14:14:02` | Canary requests succeed (`SuccessThreshold = 2`). Breaker transitions back to `StateClosed`. Normal traffic restored. |

---

## 4. Observability & Distributed Trace Breakdown

### 4.1. RED Metrics Under Failure
During the latency cascade, Prometheus RED metrics captured the severe latency divergence:

| Metric | Normal Baseline | Upstream Degraded (No Breaker) | Circuit Breaker Active (Open State) |
|---|---|---|---|
| **Throughput (RPS)** | 120 rps | 24 rps (bottlenecked) | 120 rps (fast rejection) |
| **P50 Latency** | 210ms | 9,800ms | **0.1ms** |
| **P95 Latency** | 340ms | 15,200ms | **0.4ms** |
| **P99 Latency** | 480ms | 18,500ms | **1.2ms** |
| **Active In-Flight Goroutines** | 15 - 25 | 450+ (worker exhaustion) | **8 - 14** |
| **Readiness Probe (`/ready`)** | 200 OK (2ms) | 503 Unready (timeout queuing) | **200 OK (1.8ms)** |

### 4.2. OpenTelemetry Trace Span Breakdown
Distributed tracing in Jaeger/OpenTelemetry pinpointed the exact component responsible for latency inflation:

```text
Trace ID: 7f8a91b2c3d4e5f60718293a4b5c6d7e
Total Duration: 15,248ms
Status: Error (DeadlineExceeded)

[Span] HTTP POST /api/v1/ocr/ktp ................................ 15,248ms
  ├── [Span] auth.verify_api_key ................................... 1.2ms [Ok]
  ├── [Span] rate_limit.check_token ................................ 0.1ms [Ok]
  ├── [Span] quota.check_monthly ................................... 1.8ms [Ok]
  ├── [Span] ocr.validate_image .................................... 4.5ms [Ok]
  └── [Span] ocr.engine_extract ................................ 15,240ms [Error: context.DeadlineExceeded]
        └── [Span] gemini.http_request (external Google API) ... 15,239ms [Error]
```

The trace confirmed that Lensio validation, rate limiting, and database queries accounted for **< 8ms (< 0.05%)** of total execution time. The external network call to `generativelanguage.googleapis.com` consumed **99.95%** of the request lifecycle.

---

## 5. Root Cause Analysis (5 Whys)

1. **Why did API request latency spike above 15 seconds?**  
   The upstream Gemini Vision API experienced regional latency spikes and socket read timeouts.
2. **Why did the Lensio API server become sluggish on unrelated routes?**  
   Each slow request occupied an HTTP worker goroutine and OS file descriptors for up to 15 seconds. Incoming requests queued behind blocked workers.
3. **Why didn't timeouts prevent the cascade?**  
   A static 10-second timeout still allowed hundreds of concurrent requests to hang for 10 seconds each, overwhelming the server concurrency envelope ($100\text{ req/s} \times 10\text{s} = 1,000\text{ concurrent blocked threads}$).
4. **Why did the system need a Circuit Breaker?**  
   A circuit breaker stops calling a known-failing downstream dependency altogether, short-circuiting failing calls into instant (< 1ms) responses until the provider recovers.
5. **Why are client errors isolated from the breaker?**  
   Client-side errors (bad image formats, corrupted bytes, non-KTP documents) return 400/422 and must **never** trip the circuit breaker, which only monitors true upstream service failures (`ErrOCRFailed`, `context.DeadlineExceeded`, network drops).

---

## 6. Code Fix & Architecture

### 6.1. Three-State Circuit Breaker (`services/ocr/circuit_breaker.go`)
We implemented a thread-safe, low-latency Circuit Breaker wrapping the `ocr.OCREngine` interface:

```text
   ┌────────────────────────────────────────────────────────┐
   │                                                        │
   ▼                                                        │ Successes >= SuccessThreshold
┌────────────┐   Failures >= FailureThreshold   ┌─────────┐ │
│   CLOSED   │ ───────────────────────────────> │  OPEN   │ │
└────────────┘                                  └─────────┘ │
   ▲                                                 │      │
   │                                                 │ Cooldown Elapsed (10s)
   │                                                 ▼      │
   │              Failure in Probe             ┌───────────┐│
   └────────────────────────────────────────── │ HALF-OPEN ├┘
                                               └───────────┘
```

- **Closed**: Requests pass through to upstream. Failures increment counter; successes reset counter.
- **Open**: Requests fast-fail immediately in **< 1ms** with `ocr.ErrCircuitOpen`. Cooldown timer (10s) guards recovery.
- **Half-Open**: Allows a single canary probe request through. If successful twice (`SuccessThreshold = 2`), closes breaker. If probe fails, immediately re-trips to Open.
- **Adaptive Timeout**: Automatically tightens downstream context deadline as failures mount, preventing tail latency spikes.
- **Prometheus Observability**:
  - `lensio_ocr_circuit_breaker_state`: Gauge indicating 0 (Closed), 1 (Half-Open), 2 (Open).
  - `lensio_ocr_circuit_breaker_tripped_total`: Counter tracking trips to Open.

### 6.2. Graceful HTTP 504 Degradation (`apps/api/internal/http/handlers/ocr.go`)
When `ocr.ErrCircuitOpen` is returned, the handler immediately returns:
```json
{
  "error": {
    "code": "ocr_failed",
    "message": "OCR circuit breaker is open: upstream service temporarily unavailable",
    "request_id": "req-984fc0dc-8ca0"
  }
}
```
HTTP status is **504 Gateway Timeout**, correctly signaling upstream failure to client reverse proxies and SDK retries without wasting server worker threads.

---

## 7. Verification Drill Reproduction

To reproduce this scenario locally or in CI:

```bash
# 1. Run unit test suite for CircuitBreaker state transitions
go test -race -v -run TestCircuitBreaker ./services/ocr/...

# 2. Run fast-fail benchmark (< 100ns execution, zero allocations)
go test -bench=BenchmarkCircuitBreaker -benchmem ./services/ocr/...

# 3. Run integrated failure drill Scenario E
go test -race -v -run TestDrill_ScenarioE ./apps/api/internal/http/...

# 4. Run the full unified production failure drill harness
./scripts/run-failure-drills.sh
```

---

## 8. Action Items & Lessons Learned

| Action Item | Owner | Status |
|---|---|---|
| Implement 3-state Circuit Breaker in `services/ocr/circuit_breaker.go` | Platform Eng | **Done** |
| Integrate Circuit Breaker into `services/ocr/engine.go` and `main.go` | Platform Eng | **Done** |
| Expose `lensio_ocr_circuit_breaker_state` and `tripped_total` Prometheus metrics | Observability | **Done** |
| Add automated Scenario E drill to `failure_drills_test.go` and `run-failure-drills.sh` | QA & Reliability | **Done** |
| Add `CircuitBreakerOpen` alerting rule in Prometheus (Phase 11.7) | Observability | **Scheduled (11.7)** |
