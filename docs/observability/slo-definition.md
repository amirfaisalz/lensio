# Lensio Service Level Objectives (SLO) & Error Budget Policy

> **Operational Contract & Reliability Framework**  
> *Governed by `PRD.md`, `DEVELOPMENT_TRACKING.md` (Phase 11.7), and SRE Best Practices.*

---

## 1. Executive Summary & Philosophy

Lensio provides an automated Indonesian KTP OCR extraction platform consumed by business clients in fintech, banking, e-commerce, and government compliance workflows. Because client onboarding funnels directly depend on Lensio's uptime and responsiveness, we establish explicit **Service Level Agreements (SLAs)**, **Service Level Objectives (SLOs)**, and **Service Level Indicators (SLIs)**.

### Core Philosophy
> *"Dashboards show what is happening; alerts tell us when we must act. We established an explicit 99.9% availability SLO and configured Prometheus alert rules targeting our error budget consumption rate."*

We follow the Google SRE framework:
1. **SLIs** measure real user experience directly from system telemetry.
2. **SLOs** set quantifiable reliability targets that balance user happiness against engineering velocity.
3. **Error Budgets** dictate when developers may ship new features versus when they must prioritize stability and infrastructure hardening.
4. **Actionable Alerts** page humans only when an incident consumes error budget at a dangerous velocity or threatens imminent customer downtime.

---

## 2. Service Level Objectives (SLOs) & Indicator Definitions

Lensio tracks two primary SLO dimensions: **Availability** and **Latency**.

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. Availability Objective: 99.9% Success Rate (Three Nines)                │
│    SLI: (Total Requests - 5xx Errors) / Total Eligible Requests >= 0.999    │
│    Rolling Window: 30 Calendar Days (43,200 minutes)                       │
│    Permitted Downtime / Error Budget: 43.2 minutes per 30-day window        │
├─────────────────────────────────────────────────────────────────────────────┤
│ 2. Latency Objective (API Gateway / Validation Overhead): P95 < 500ms       │
│    SLI: 95% of requests complete within 500ms                              │
├─────────────────────────────────────────────────────────────────────────────┤
│ 3. Latency Objective (End-to-End OCR Pipeline): P95 < 2,000ms               │
│    SLI: 95% of successful OCR requests complete within 2.0s                │
│    Circuit Breaker Bound: Fast-fails upstream timeouts in <= 100ms (504)    │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.1 Availability SLI & SLO Specification

| Component | Target (SLO) | Permitted Unavailability | Measurement Window |
|---|---|---|---|
| **Production API (`api.lensio.dev`)** | **99.9%** | **43.2 minutes / month** | 30-Day Rolling Window |
| **Weekly Error Budget** | 99.9% | 10.08 minutes / week | 7-Day Rolling Window |
| **Daily Error Budget** | 99.9% | 1.44 minutes / day | 24-Hour Rolling Window |

#### SLI Formula
$$\text{SLI}_{\text{availability}} = \frac{\sum \text{rate}(http\_requests\_total\{status\_code !~ "5.."}[30d])}{\sum \text{rate}(http\_requests\_total[30d])} \ge 0.999$$

#### Eligible vs. Excluded Traffic
- **Included (Counts against Error Budget)**:
  - Any HTTP status code in the `5xx` range (`500 Internal Server Error`, `502 Bad Gateway`, `503 Service Unavailable`, `504 Gateway Timeout`).
  - Unhandled panics or crashes caught by HTTP recovery middleware.
  - Failures in PostgreSQL connection pool resulting in 500s.
  - Upstream Vision AI exhaustion or timeouts resulting in 504s when the circuit breaker trips.
- **Excluded (Client-Side Errors - Does NOT count against Error Budget)**:
  - `400 Bad Request`: Malformed multipart payloads, unreadable image headers, missing form fields.
  - `401 Unauthorized` / `403 Forbidden`: Invalid or revoked API keys, missing RBAC/SpiceDB permissions.
  - `404 Not Found`: Calls to undefined endpoints.
  - `422 Unprocessable Entity`: Synthetic or unparseable KTP cards failing deterministic NIK validation.
  - `429 Too Many Requests`: Client exceeding their plan's rate limit or monthly quota.

### 2.2 Latency SLI & SLO Specification

1. **API Gateway & Boundary Validation Latency**:
   - **Target**: 95% of incoming requests served in $\le 500\text{ms}$ over any 5-minute window.
   - **PromQL**:
     ```promql
     histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) <= 0.5
     ```
2. **Full OCR Processing Pipeline Latency**:
   - **Target**: 95% of successful `/api/v1/ocr/ktp` extractions served in $\le 2,000\text{ms}$ over any 5-minute window.
   - **PromQL**:
     ```promql
     histogram_quantile(0.95, sum(rate(ocr_duration_seconds_bucket{stage="overall",status="completed"}[5m])) by (le)) <= 2.0
     ```
   - **Resilience Guarantee**: If the upstream Vision AI (Gemini 2.0 Flash) latency spikes beyond thresholds, the adaptive circuit breaker trips to `StateOpen` and returns an immediate `504 Gateway Timeout` in under `100ms`, shielding application worker pools from thread exhaustion.

---

## 3. Error Budget Consumption & Multi-Burn-Rate Alerting

### 3.1 Mathematical Derivation of Burn Rate

The 30-day error budget for a 99.9% SLO is:
$$\text{Error Budget } (EB) = 1.0 - 0.999 = 0.001 \quad (0.1\%)$$

A **Burn Rate ($B$)** of $1.0\times$ means the service consumes 100% of its monthly error budget evenly across exactly 30 days ($43.2$ minutes of total outage).

$$\text{Burn Rate } (B) = \frac{\text{Observed Error Rate}}{\text{Allowed Error Budget}} = \frac{e}{0.001}$$

- If the service has a $1\%$ error rate over a window, $B = \frac{0.01}{0.001} = 10.0\times$.
- If the service has a $100\%$ total outage ($e = 1.0$), $B = \frac{1.0}{0.001} = 1000\times$, exhausting the entire 30-day budget in just:
  $$\frac{43.2 \text{ minutes}}{1000} \approx 43.2 \text{ seconds}$$

### 3.2 Multi-Window Multi-Burn-Rate Alerting Matrix

To avoid false alarms from transient spikes while catching catastrophic failures before customers escalate, Lensio adopts the **Multi-Window Multi-Burn-Rate** alerting standard (Google SRE Workbook Chapter 5):

| Alert Tier | Burn Rate ($B$) | Budget Consumed | Time to 100% Exhaustion | Long Window | Short Window | Action / Notification |
|---|---|---|---|---|---|---|
| **P1 - Critical (Page)** | **14.4x** | 2.0% in 1 hour | **50 hours (2 days)** | 1 hour | 5 minutes | PagerDuty On-Call Page, immediate emergency response |
| **P1 - Critical (Page)** | **6.0x** | 5.0% in 6 hours | **120 hours (5 days)** | 6 hours | 30 minutes | PagerDuty On-Call Page, incident triage |
| **P2 - Warning (Ticket)** | **2.0x** | 10.0% in 36 hours | **360 hours (15 days)** | 36 hours | 2 hours | Slack `#alerts-prod` + Jira Ticket within 4 hours |
| **P3 - Low (Review)** | **1.0x** | 10.0% in 3 days | **720 hours (30 days)** | 72 hours | 6 hours | Weekly Reliability Review & Backlog Triage |

#### Dual-Window Validation Rule
An alert fires **only if both the long window AND the short window exceed the burn rate threshold**.
- The **long window** ensures statistical significance and high precision.
- The **short window** ensures the alert resets immediately once the incident is resolved, avoiding alert fatigue.

---

## 4. Actionable Prometheus Alerting Matrix

Every alert defined in `infra/observability/prometheus/alerting_rules.yml` directly corresponds to an SLI degradation, has an assigned owner, and links directly to a tested incident runbook:

| Alert Name | Condition (PromQL) | Duration (`for`) | Severity | SLI Impact | Primary Responder | Runbook Reference |
|---|---|---|---|---|---|---|
| **`HighErrorRate`** | `(sum(rate(http_requests_total{status_code=~"5.."}[2m])) / clamp_min(sum(rate(http_requests_total[2m])), 0.001)) * 100 > 1.0` | `2m` | **Critical (P1)** | Availability SLO (< 99.9%) | Platform On-Call | [`docs/rollback.md`](../rollback.md)<br>[`INC-20260912-04`](../incidents/INC-20260912-04-production-regression-drill.md) |
| **`P95LatencyBreached`** | `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 0.5` | `5m` | **Warning (P2)** | Latency SLO (> 500ms) | Platform On-Call | [`INC-20260912-05`](../incidents/INC-20260912-05-ocr-provider-latency-cascade.md) |
| **`CircuitBreakerOpen`** | `lensio_ocr_circuit_breaker_state == 2` | `1m` | **Critical (P1)** | OCR Pipeline Availability | Vision AI On-Call | [`INC-20260912-05`](../incidents/INC-20260912-05-ocr-provider-latency-cascade.md) |
| **`RateLimitSurge`** | `(sum(rate(http_requests_total{status_code="429"}[5m])) / clamp_min(sum(rate(http_requests_total[5m])), 0.001)) * 100 > 25.0` | `2m` | **Warning (P2)** | Security & Traffic Hygiene | Security Operations | [`ADR-003`](../decisions/ADR-003-rate-limiting-and-quota-architecture.md) |
| **`FastErrorBudgetBurnRate`** | Multi-window 14.4x burn rate over 1h & 5m windows | `2m` | **Critical (P1)** | Error Budget Depletion (2%/hr) | Platform On-Call | [`docs/rollback.md`](../rollback.md) |
| **`DatabaseConnectionPoolSaturation`** | `(db_pool_connections_in_use / clamp_min(db_pool_connections_open, 1)) * 100 > 85.0` | `3m` | **Warning (P2)** | Database Exhaustion Risk | Platform On-Call | [`INC-20260912-02`](../incidents/INC-20260912-02-database-outage.md) |

---

## 5. Operational Governance & Error Budget Policies

To maintain operational discipline, Lensio enforces strict error budget governance across product and engineering teams:

### 5.1 The Error Budget Freeze Policy
The remaining 30-day error budget determines engineering prioritization:

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│ Healthy (100% - 50% Budget Remaining): Normal Feature Velocity              │
│ - Product and engineering teams ship features, optimizations, experiments.  │
├─────────────────────────────────────────────────────────────────────────────┤
│ Yellow State (50% - 20% Budget Remaining): Heightened Reliability Scrutiny  │
│ - Mandatory canary rollouts (10% traffic for 15 minutes before 100%).       │
│ - Stricter load test benchmarks required before merging PRs.                │
├─────────────────────────────────────────────────────────────────────────────┤
│ Red State (20% - 10% Budget Remaining): Operational Slowdown                │
│ - Non-essential production deployments require VP / Staff approval.         │
│ - Team shifts 50% of sprint capacity to reliability and automated testing.  │
├─────────────────────────────────────────────────────────────────────────────┤
│ Budget Depleted (< 10% Budget Remaining): HARD FEATURE FREEZE               │
│ - ZERO product feature releases permitted into production.                  │
│ - 100% of engineering effort dedicated to bug fixes, resilience, and root-  │
│   cause remediation until the 30-day rolling budget recovers above 20%.     │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Planned Maintenance Policy
- Planned maintenance windows are scheduled only between **02:00 - 04:00 Western Indonesia Time (WIB / UTC+7)** on Sunday mornings.
- Planned maintenance must be announced to customers via `status.lensio.dev` at least 72 hours in advance.
- Maintenance traffic must be marked with an HTTP header or routed through maintenance bypass flags to prevent polluting client-facing SLO counters.

### 5.3 On-Call Escalation Paths & Triage Protocol

1. **T0 (Alert Fired)**:
   - Prometheus Alertmanager evaluates rule condition and notifies PagerDuty / Slack `#alerts-prod`.
2. **T+5m (Triage & Acknowledgement)**:
   - On-Call engineer acknowledges alert, checks Grafana dashboard (`https://grafana.lensio.dev/d/lensio-slo-business`), and verifies trace IDs in Grafana Tempo.
3. **T+10m (Mitigation Decision)**:
   - If alert is `HighErrorRate` following a new container deployment, execute immediate rollback (< 60s target per `docs/rollback.md` and `INC-20260912-04`).
   - If alert is `CircuitBreakerOpen`, verify provider status (e.g. Google Gemini status page) and check if fallback mock/secondary provider should be enabled.
   - If alert is `DatabaseConnectionPoolSaturation`, verify slow queries in pg_stat_activity and scale connection pool limits or replica endpoints.
4. **T+24h (Post-Mortem & Blameless Review)**:
   - Author a blameless post-mortem using the template in `docs/incidents/README.md`.
   - Calculate exact error budget percentage consumed during the incident and publish to the engineering status board.

---

## 6. Verification & Automated Promtool Tests

Prometheus rules and configuration in Lensio are fully verified through automated test suites:

### 6.1 Syntax & Config Verification
```bash
# Check alert rule YAML syntax and PromQL semantics
docker run --rm --entrypoint promtool \
  -v "$(pwd)/infra/observability/prometheus:/etc/prometheus" \
  prom/prometheus:latest check rules /etc/prometheus/alerting_rules.yml

# Check Prometheus server configuration and rule file inclusion
docker run --rm --entrypoint promtool \
  -v "$(pwd)/infra/observability/prometheus:/etc/prometheus" \
  prom/prometheus:latest check config /etc/prometheus/prometheus.yml
```

### 6.2 Promtool Unit Test Execution
The rule test suite (`infra/observability/prometheus/alerting_rules_test.yml`) simulates metric time series and validates that alerts fire deterministically with correct labels, annotations, and durations:

```bash
docker run --rm --entrypoint promtool \
  -v "$(pwd)/infra/observability/prometheus:/etc/prometheus" \
  -w /etc/prometheus \
  prom/prometheus:latest test rules alerting_rules_test.yml
```
