# NusaID Observability, Telemetry & Operations Runbook

> Production guide for OpenTelemetry tracing, Prometheus RED metrics, Grafana dashboards, and structured logging.  
> Governed by `PRD Section 16` and `AGENTS.md` (Mandatory Rule 4).

---

## 1. Observability Architecture Overview

NusaID implements unified observability across three pillars: **Traces**, **Metrics**, and **Logs**, correlating telemetry through universal request and trace identifiers:

```text
HTTP Request (Inbound)
      │
      ▼
[RequestID Middleware]   ──► Generates 'req_01JABC...' / Injects X-Request-ID
      │
      ▼
[OpenTelemetry Tracer]   ──► Injects W3C traceparent (trace_id, span_id)
      │
      ▼
[Log/slog Logger]        ──► Structured JSON logs correlated with trace_id & request_id
      │
      ▼
[Prometheus Metrics]     ──► Records latency histograms, error counters, OCR stats
      │
      ▼
[Export Pipeline]
      ├── Prometheus /metrics endpoint (Scraped by Prometheus server)
      ├── OTLP gRPC Exporter (Pushes spans to Grafana Tempo / OpenTelemetry Collector)
      └── Standard Output Stream (Harvested by Azure Container Apps Log Analytics)
```

---

## 2. Distributed Tracing (`OpenTelemetry`)

The Go backend utilizes the official OpenTelemetry Go SDK (`go.opentelemetry.io/otel`). Every incoming request creates a root span with contextual child spans:

### Span Hierarchy

```text
root: http.request (route: POST /api/v1/ocr/ktp, method: POST)
│
├── child: auth.verify_api_key
│   └── child: db.query (SELECT api_keys WHERE hash = $1)
│
├── child: quota.check
│   └── child: db.query (SELECT count(usage_records) WHERE org_id = $1)
│
├── child: ocr.process_pipeline
│   ├── child: image.validate (MIME & magic bytes)
│   ├── child: ocr_engine.extract (External call to Gemini 2.0 Flash / Mock)
│   └── child: ktp.normalize_and_validate (NIK regex, province code, DOB)
│
└── child: usage.meter_async (Enqueues record to buffered channel)
```

### Trace Context Propagation
- Accepts incoming `traceparent` headers conforming to the W3C Trace Context specification.
- Injects `X-Request-ID` and `traceparent` into outbound responses for client-side correlation.

---

## 3. RED Metrics & Prometheus Endpoint

NusaID exposes a standard Prometheus metrics scraper route at `GET /metrics`.

### Core RED Metrics (Rate, Errors, Duration)

| Metric Name | Type | Labels | Description |
|---|---|---|---|
| `nusaid_http_requests_total` | Counter | `route`, `method`, `status` | Total incoming HTTP requests partitioned by status code. |
| `nusaid_http_request_duration_seconds` | Histogram | `route`, `method` | End-to-end HTTP request latency distribution (P50, P90, P95, P99). |
| `nusaid_http_errors_total` | Counter | `route`, `error_code` | Count of application errors (`rate_limit_exceeded`, `ocr_failed`, `invalid_document`). |

### Business & Operational Metrics

| Metric Name | Type | Labels | Description |
|---|---|---|---|
| `nusaid_ocr_requests_total` | Counter | `provider`, `status`, `doc_type` | Total OCR requests handled. |
| `nusaid_ocr_confidence` | Histogram | `doc_type` | Document confidence score distribution (0.0 to 1.0). |
| `nusaid_ocr_duration_seconds` | Histogram | `provider` | Pure OCR vision provider inference latency. |
| `nusaid_rate_limit_hits_total` | Counter | `org_id`, `plan` | Count of rate limit violations (HTTP 429). |
| `nusaid_quota_exceeded_total` | Counter | `org_id`, `plan` | Count of monthly quota blocks. |
| `nusaid_db_connections` | Gauge | `state` | PostgreSQL connection pool status (`idle`, `active`, `total`). |

---

## 4. Service Level Objectives (SLO) & Alerts

NusaID measures performance against two strict production SLOs:

```text
┌─────────────────────────────────────────────────────────────┐
│ 1. Availability Objective: 99.0% Success Rate (SLA Tier)     │
│    SLI: (2xx + 4xx valid) / (Total Requests - 429) >= 0.99  │
│    Error Budget: 1.0% unhandled 5xx errors per 30-day window│
├─────────────────────────────────────────────────────────────┤
│ 2. Latency Objective: P95 < 2,000ms                         │
│    SLI: 95% of successful OCR requests complete in < 2.0s   │
└─────────────────────────────────────────────────────────────┘
```

### Pre-configured Alert Rules (Prometheus / Alertmanager):

```yaml
groups:
  - name: nusaid_alerts
    rules:
      - alert: NusaIDHigh5xxErrorRate
        expr: (sum(rate(nusaid_http_requests_total{status=~"5.."}[5m])) / sum(rate(nusaid_http_requests_total[5m]))) * 100 > 1.0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "NusaID API 5xx error rate exceeds 1% error budget"
          runbook: "docs/rollback.md"

      - alert: NusaIDHighP95Latency
        expr: histogram_quantile(0.95, sum(rate(nusaid_http_request_duration_seconds_bucket[5m])) by (le)) > 2.0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "NusaID P95 latency degraded beyond 2.0s threshold"
          runbook: "docs/incidents/INC-20260912-01-ocr-provider-timeout.md"

      - alert: NusaIDDatabaseDown
        expr: nusaid_db_connections{state="active"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "PostgreSQL connection pool exhausted or database unreachable"
          runbook: "docs/incidents/INC-20260912-02-database-outage.md"
```

---

## 5. Structured Logging Guidelines (`log/slog`)

Application logging strictly uses Go's standard `log/slog` emitting newline-delimited JSON (NDJSON) to standard output.

### Log Schema Specification:
```json
{
  "time": "2026-09-12T01:50:00.123Z",
  "level": "INFO",
  "msg": "ocr_extraction_completed",
  "request_id": "req_01JABC123456",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "org_id": "org_01JXYZ789",
  "status_code": 200,
  "confidence": 0.98,
  "latency_ms": 1420
}
```

### Privacy & PII Scrubbing Policy:
Every logger invocation must pass the PII audit rule:
- **NEVER log citizen NIK, names, dates of birth, addresses, or photos.**
- When errors occur during extraction, log the error reason (e.g. `err="nik validation failed: invalid province code"`) without logging the invalid number itself.

---

## 6. Accessing Observability in Production

- **Prometheus Metrics**: `curl -s https://api.nusaid.com/metrics`
- **Health & Readiness Check**:
  ```bash
  curl -i https://api.nusaid.com/health
  curl -i https://api.nusaid.com/ready
  ```
- **Grafana Dashboard**: Accessible at `https://grafana.nusaid.com` with pre-built panels:
  - Panel 1: Ingestion Rate (RPS by route)
  - Panel 2: HTTP Latency Heatmap & P95 Line
  - Panel 3: 4xx / 5xx Error Distribution
  - Panel 4: OCR Provider Availability & Latency
  - Panel 5: Database Connection Pool Stats
