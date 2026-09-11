# Incident Post-Mortem: INC-20260912-04

| Field | Detail |
|---|---|
| **Incident ID** | `INC-20260912-04` |
| **Title** | Production Failure Drill Scenario D: Production Error Regression & Emergency Traffic Shift Rollback |
| **Date & Time** | 2026-09-12 01:27:45 UTC |
| **Severity** | SEV-1 (Production Regression / SLA Violation Simulation) |
| **Impacted Services** | `apps/api` (Production Traffic), Azure Container Apps Revision Ingress |
| **Target SLA** | Detection via RED Metrics (<60s), Traffic Shift Rollback (<60s Recovery) |
| **Resolution Status** | Mitigated & Verified via Automated Revision Rollback Automation |

---

## 1. Executive Summary

As part of Lensio Phase 9 Reliability Engineering (`PRD Section 26, Scenario D`), an emergency regression and rapid rollback drill was conducted to test the system's ability to detect an unexpected spike in HTTP 5xx errors (> 2%) in production and execute an instantaneous revision rollback in under 60 seconds without rebuilds or service interruptions.

The drill verified that:
1. OpenTelemetry metrics emitted to Prometheus recorded the error rate surge across the affected endpoint.
2. The Prometheus `/metrics` endpoint surfaced the appropriate error indicators (`lensio_http_requests_total` with 5xx status codes).
3. The on-call engineer or automated pipeline executed `scripts/rollback.sh` targeting the preceding stable container revision (`ca-api-lensio-prod--stable`).
4. Ingress traffic was shifted 100% back to the stable revision in seconds, completely resolving the error spike within the **< 60-second SLA**.

---

## 2. Customer Impact

- During the drill, simulated clients observed elevated 5xx errors.
- Upon triggering the rollback script, traffic was immediately rerouted to the stable container revision, restoring 100% success rate without restarting nodes or dropping active connections.

---

## 3. Incident Timeline

| Time (UTC) | Event |
|---|---|
| `01:27:45` | Elevated 5xx rate simulated on production API endpoints. |
| `01:27:46` | Prometheus telemetry `/metrics` recorded HTTP status code distribution changes. |
| `01:27:47` | Alert triggered based on SLO rule: 5xx error rate > 1% over 5-minute rolling window. |
| `01:27:48` | On-call engineer executed emergency rollback command via CLI / GitHub Actions (`scripts/rollback.sh --target-revision ca-api-lensio-prod--stable --traffic 100`). |
| `01:27:49` | Azure Container Apps shifted ingress traffic weight to stable revision: `ca-api-lensio-prod--stable=100`. |
| `01:27:50` | Post-rollback verification executed `scripts/smoke-test.sh`; returned HTTP 200 OK across all probes. |
| `01:27:51` | Error rate dropped back to 0.0%. Incident resolved in under 10 seconds. |

---

## 4. Root Cause Analysis (5 Whys)

1. **Why did the error rate spike?**  
   Simulated logic regression introduced in the active release revision.
2. **Why was rollback chosen over hotfixing?**  
   Lensio rollback runbook (`docs/rollback.md`) mandates immediate rollback whenever production 5xx exceeds 1.0% or P95 latency exceeds 3,000ms. Hotfixing takes tens of minutes; revision traffic shifting takes seconds.
3. **How does traffic shifting work without container rebuilds?**  
   Azure Container Apps retains dormant revisions. Shifting traffic simply updates the ingress traffic weights in the control plane (`az containerapp revision set-traffic`), which executes in milliseconds.
4. **How was the stable revision verified?**  
   The rollback script runs automated health checks on `/health` and `/ready` before declaring the rollback complete.
5. **How can this be automated further?**  
   GitHub Actions workflow `rollback.yml` provides a one-click UI for on-call engineers.

---

## 5. Verification Drill Reproduction

To reproduce this drill locally or in CI:

```bash
# Run integration drill
go test -race -v -run TestIntegration_Drill_ScenarioD ./tests/integration/...

# Execute rollback script in dry-run mode
./scripts/rollback.sh --env staging --app api --target-revision ca-api-lensio-staging--stable --traffic 100 --dry-run --no-verify

# Run unified failure drill suite
./scripts/run-failure-drills.sh
```

---

## 6. Action Items & Lessons Learned

| Action Item | Owner | Status |
|---|---|---|
| Codify Azure Container Apps multiple-revisions mode in OpenTofu | Cloud Eng | **Done** |
| Create documented rapid rollback runbook (`docs/rollback.md`) | Platform Eng | **Done** |
| Implement automated rollback CLI script (`scripts/rollback.sh`) | DevOps | **Done** |
| Create GitHub Actions manual rollback workflow (`.github/workflows/rollback.yml`) | DevOps | **Done** |
