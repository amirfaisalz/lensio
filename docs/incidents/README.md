# Incident Reports & Post-Mortems

> Blameless post-mortem reports generated from production failure drills and simulated incidents (`PRD Section 26`).

---

## Simulated Production Drills (Phase 9)

| Incident ID | Date | Severity | Title | Scenario |
|---|---|---|---|---|
| [`INC-20260912-01`](./INC-20260912-01-ocr-provider-timeout.md) | 2026-09-12 | SEV-2 | Vision AI OCR Provider Latency & Timeout Simulation | **Scenario A**: OCR Provider Failure / Timeout |
| [`INC-20260912-02`](./INC-20260912-02-database-outage.md) | 2026-09-12 | SEV-1 | PostgreSQL Database Outage & Readiness Probe Isolation | **Scenario B**: Database Unavailable |
| [`INC-20260912-03`](./INC-20260912-03-broken-deployment-smoke-test.md) | 2026-09-12 | SEV-2 | Broken Deployment Simulation & Automated Smoke Test Promotion Halt | **Scenario C**: Broken Deployment Smoke Test |
| [`INC-20260912-04`](./INC-20260912-04-production-regression-drill.md) | 2026-09-12 | SEV-1 | Production Error Regression & Emergency Traffic Shift Rollback | **Scenario D**: Production Regression Drill (<60s SLA) |

---

## Executing Failure Drills

All failure drills can be executed automatically and deterministically using the repository test runner:

```bash
# Execute full automated failure drill suite
./scripts/run-failure-drills.sh

# Run individual scenario tests
go test -race -v -run TestDrill_ScenarioA ./apps/api/internal/http/...
go test -race -v -run TestDrill_ScenarioB ./apps/api/internal/http/...
go test -race -v -run TestIntegration_Drill_ScenarioC ./tests/integration/...
go test -race -v -run TestIntegration_Drill_ScenarioD ./tests/integration/...
```
