# NusaID Rollback Runbook & Procedures

> Standard operating procedure for executing emergency rollbacks on NusaID deployments in **under 60 seconds**.

---

## 1. Rollback Mandate & Triggers

An immediate revision rollback must be initiated if any of the following criteria are met within 15 minutes of a deployment:

1. **Smoke Test Failure**: Automated smoke test fails on `/health` or `/ready`.
2. **Error Rate Spike**: HTTP 5xx error rate exceeds **1.0%** over a 5-minute rolling window.
3. **Latency Degradation**: P95 latency exceeds **3,000ms** (SLA baseline: <2,000ms).
4. **Data Corruption / Contract Breakage**: OCR pipeline returns malformed response structures.

---

## 2. Architecture: Revision Traffic Shifting

Azure Container Apps supports multiple concurrent revisions. When a new container image is deployed, the preceding revision remains instantiated in a dormant or running state. 

Rolling back does **not** re-build or re-download container layers; it executes an instantaneous routing metadata update shifting 100% of ingress traffic back to the preceding stable revision:

```text
               Cloudflare Ingress
                       │
                       ▼
             Azure Container App
            ┌───────────────────┐
            │   Traffic Switch  │
            └─────────┬─────────┘
                      │
     ┌────────────────┴────────────────┐
     │ 0% Traffic                      │ 100% Traffic (ROLLBACK)
     ▼                                 ▼
[Revision: 1.4.1 (Faulty)]        [Revision: 1.4.0 (Stable)]
```

---

## 3. Method A: Automated GitHub Actions Rollback (Recommended)

1. Navigate to **Actions** -> **NusaID Emergency Rollback** in GitHub repository.
2. Click **Run workflow**.
3. Fill in the parameters:
   - **Environment**: `production` (or `staging`)
   - **Component**: `api` (or `dashboard` / `all`)
   - **Target Revision**: Enter previous stable revision name (e.g., `ca-api-nusaid-prod--1-4-0`)
   - **Traffic Percentage**: `100`
   - **Incident Reason**: Brief description of the observed symptom.
4. Click **Run workflow**.
5. The workflow executes `scripts/rollback.sh` and verifies `/health` and `/ready` within 30 seconds.

---

## 4. Method B: Emergency Azure CLI Rollback (< 30 Seconds)

If GitHub Actions is unreachable or experiencing delays, an engineer with Azure access can execute the CLI command directly:

```bash
# 1. List active revisions to identify previous stable revision
az containerapp revision list \
  --name ca-api-nusaid-production \
  --resource-group rg-nusaid-production \
  --query "[].{Name:name, Created:createdTime, Traffic:trafficWeight, Active:active}" \
  --output table

# 2. Shift 100% traffic to stable revision immediately
az containerapp revision set-traffic \
  --name ca-api-nusaid-production \
  --resource-group rg-nusaid-production \
  --revision-weight ca-api-nusaid-prod--<stable-revision>=100

# 3. Verify health probe
curl -f https://api.nusaid.com/health
curl -f https://api.nusaid.com/ready
```

Alternatively, run the automated script directly from the repository root:

```bash
./scripts/rollback.sh \
  --env production \
  --app api \
  --target-revision ca-api-nusaid-prod--<stable-revision> \
  --traffic 100
```

---

## 5. Post-Rollback Verification Checklist

- [ ] Confirm `curl https://api.nusaid.com/health` returns `{"status":"ok"}`.
- [ ] Confirm `curl https://api.nusaid.com/ready` returns `{"status":"ready","database":"connected"}`.
- [ ] Execute smoke test: `./scripts/smoke-test.sh https://api.nusaid.com`.
- [ ] Check Grafana RED dashboard: Verify 5xx error rate drops back to 0%.
- [ ] Log incident post-mortem in `docs/incidents/` documenting root cause.
