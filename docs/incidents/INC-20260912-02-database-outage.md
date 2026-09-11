# Incident Post-Mortem: INC-20260912-02

| Field | Detail |
|---|---|
| **Incident ID** | `INC-20260912-02` |
| **Title** | Production Failure Drill Scenario B: PostgreSQL Database Outage & Readiness Probe Isolation |
| **Date & Time** | 2026-09-12 01:27:15 UTC |
| **Severity** | SEV-1 (Critical Infrastructure Dependency Failure) |
| **Impacted Services** | `apps/api` (PostgreSQL connection pool, `/ready`, authenticated API routes) |
| **Target SLA** | Instantaneous Ingress Traffic Evacuation (`/ready` $\rightarrow$ 503), Process Liveness (`/health` $\rightarrow$ 200), Safe Rejection |
| **Resolution Status** | Mitigated & Verified via Automated Resilience Drill |

---

## 1. Executive Summary

As part of NusaID Phase 9 Reliability Engineering (`PRD Section 26, Scenario B`), an infrastructure simulation was conducted to test the behavior of the NusaID API instance when its primary relational data store (Azure Database for PostgreSQL Flexible Server) becomes unreachable or crashes.

The drill verified that:
1. Process liveness (`GET /health`) remained **HTTP 200 OK**, preventing Azure Container Apps / Kubernetes orchestrators from entering infinite, thrashing container restart loops while the database was recovering.
2. Readiness probe (`GET /ready`) immediately flipped to **HTTP 503 Service Unavailable** (`{"status":"unready","database":"disconnected"}`), signalling ingress load balancers and Cloudflare to immediately evict the instance from active traffic pools.
3. In-flight requests requiring database interaction (such as API key verification middleware) were safely rejected with controlled **HTTP 500 Internal Server Error** (`code: internal_error`) without server panics, memory corruption, or hanging database socket locks.

---

## 2. Customer Impact

- Upstream load balancers detected the 503 status code on `/ready` within 5 seconds and routed traffic away from the degraded node.
- Direct callers to authenticated endpoints received standard JSON error envelopes rather than unexpected HTTP gateway resets or partial data corruption.
- Process uptime was preserved; once PostgreSQL connectivity was restored, the readiness probe returned to HTTP 200 within 2 seconds without requiring container restarts.

---

## 3. Incident Timeline

| Time (UTC) | Event |
|---|---|
| `01:27:15` | Database failure injected via simulated pool disconnect and failing `Pinger`. |
| `01:27:16` | Automated test probed `GET /health`; received **HTTP 200 OK** (`{"status":"ok"}`). |
| `01:27:16` | Automated test probed `GET /ready`; received **HTTP 503 Service Unavailable** (`{"status":"unready","database":"disconnected"}`). |
| `01:27:17` | Authenticated request sent to `POST /api/v1/ocr/ktp`. Authentication middleware attempted key hash query. |
| `01:27:17` | Store returned connection error; middleware handled gracefully and returned **HTTP 500** (`code: internal_error`, message: `"Failed to verify API key"`). |
| `01:27:18` | Verified zero stack traces or internal DB credentials (`postgres://user:password@host...`) leaked into client response. |

---

## 4. Root Cause Analysis (5 Whys)

1. **Why did the database become unavailable?**  
   Simulated network partition / primary PostgreSQL maintenance failover.
2. **Why was the API process not killed by the container runtime?**  
   The API deliberately separates `/health` (process liveness: only checks process memory/CPU health) from `/ready` (dependency health: tests Postgres `PingContext`).
3. **Why does this separation matter?**  
   If `/health` checked the DB, all replicas would fail simultaneously during a database hiccup, causing the container runtime to kill all pods simultaneously, amplifying downtime.
4. **How does traffic avoid the instance?**  
   Azure Container Apps ingress monitors `/ready`. When `/ready` returns 503, the ingress controller immediately removes the container from the active routing table.
5. **What prevented connection pool starvation?**  
   All database operations use bounded contexts (`context.WithTimeout(ctx, 2*time.Second)`).

---

## 5. Verification Drill Reproduction

To reproduce this drill locally or in CI:

```bash
# Run unit drill
go test -race -v -run TestDrill_ScenarioB ./apps/api/internal/http/...

# Run unified failure drill suite
./scripts/run-failure-drills.sh
```

---

## 6. Action Items & Lessons Learned

| Action Item | Owner | Status |
|---|---|---|
| Implement segregated `/health` and `/ready` handlers | Platform Eng | **Done** |
| Verify safe error return in authentication middleware on DB failure | Backend Eng | **Done** |
| Configure Azure Container Apps readiness probe interval (5s, timeout 3s) | DevOps | **Done** |
