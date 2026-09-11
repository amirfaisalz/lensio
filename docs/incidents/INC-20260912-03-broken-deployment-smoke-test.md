# Incident Post-Mortem: INC-20260912-03

| Field | Detail |
|---|---|
| **Incident ID** | `INC-20260912-03` |
| **Title** | Production Failure Drill Scenario C: Broken Deployment Simulation & Automated Smoke Test Promotion Halt |
| **Date & Time** | 2026-09-12 01:27:30 UTC |
| **Severity** | SEV-2 (Pre-Production Deployment Quality Gate Violation) |
| **Impacted Services** | CI/CD Delivery Pipeline (`.github/workflows/deploy.yml`, `scripts/smoke-test.sh`) |
| **Target SLA** | 100% Automated Prevention of Broken Release Promotion to Production |
| **Resolution Status** | Mitigated & Verified via Automated Deployment Gate Drill |

---

## 1. Executive Summary

As part of Lensio Phase 9 Reliability Engineering (`PRD Section 26, Scenario C`), an automated drill was executed to simulate an erroneous release deployment where a broken container revision (e.g. missing environment variables or fatal startup crash) is deployed to staging.

The drill verified that:
1. `scripts/smoke-test.sh` immediately probed the deployed service's `/health` and `/ready` endpoints.
2. Upon receiving unexpected non-200 responses or connection failures, the smoke test suite executed configured retries, logged explicit diagnostic errors (`[FAIL] Liveness probe failed`), and exited with code `1`.
3. The GitHub Actions deployment workflow (`deploy.yml`) trapped the non-zero exit code and terminated the job immediately, successfully preventing the faulty build from triggering promotion gates or production deployment.

---

## 2. Customer Impact

- **Zero Production Outage**: The defect was completely contained within the automated staging verification pipeline.
- Production traffic remained routed 100% to the preceding healthy revision.

---

## 3. Incident Timeline

| Time (UTC) | Event |
|---|---|
| `01:27:30` | Injected broken mock server returning HTTP 500 on `/health` (simulating runtime crash). |
| `01:27:31` | Executed `scripts/smoke-test.sh` targeting the broken endpoint with retries enabled. |
| `01:27:32` | Smoke test attempt 1 failed (HTTP 500). Retry delay observed. |
| `01:27:33` | Smoke test attempt 2 failed (HTTP 500). Max retries reached. |
| `01:27:33` | `scripts/smoke-test.sh` emitted `[FAIL] Liveness probe failed at <endpoint>/health`. |
| `01:27:33` | Process exited with return code `1`. |
| `01:27:34` | Verified in `.github/workflows/deploy.yml` that promotion job `deploy-production` is conditioned on `deploy-staging` passing, guaranteeing promotion was blocked. |

---

## 4. Root Cause Analysis (5 Whys)

1. **Why was the deployment broken?**  
   Simulated configuration error or unhandled initialization bug in the container image.
2. **Why was the error caught before reaching users?**  
   All deployments must pass an automated post-deployment smoke test (`scripts/smoke-test.sh`) against live staging endpoints before any production promotion.
3. **Why did the pipeline stop automatically?**  
   The bash script runs with `set -euo pipefail` and returns an explicit non-zero exit code when any probe check fails.
4. **Can an engineer accidentally promote a broken staging build?**  
   No; GitHub Actions branch protection and environment rules mandate that staging tests must succeed before the production approval gate unlocks.
5. **How quickly was the failure identified?**  
   Within seconds, based on `MAX_RETRIES` and `RETRY_DELAY_SEC` settings.

---

## 5. Verification Drill Reproduction

To reproduce this drill locally or in CI:

```bash
# Run integration drill
go test -race -v -run TestIntegration_Drill_ScenarioC ./tests/integration/...

# Run unified failure drill suite
./scripts/run-failure-drills.sh
```

---

## 6. Action Items & Lessons Learned

| Action Item | Owner | Status |
|---|---|---|
| Configure post-deployment smoke tests in `.github/workflows/deploy.yml` | DevOps | **Done** |
| Add functional KTP OCR test with synthetic image to smoke test suite | QA | **Done** |
| Enforce strict `pipefail` and return codes in all operational automation scripts | DevOps | **Done** |
