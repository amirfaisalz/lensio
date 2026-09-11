# Incident Post-Mortem: INC-20260912-01

| Field | Detail |
|---|---|
| **Incident ID** | `INC-20260912-01` |
| **Title** | Production Failure Drill Scenario A: Vision AI OCR Provider Latency & Timeout Simulation |
| **Date & Time** | 2026-09-12 01:27:00 UTC |
| **Severity** | SEV-2 (Degraded User Experience - OCR Pipeline) |
| **Impacted Services** | `apps/api` (`POST /api/v1/ocr/ktp`), `services/ocr` |
| **Target SLA** | Controlled Error Response (<2,000ms), Zero PII or Credential Leaks |
| **Resolution Status** | Mitigated & Verified via Automated Resilience Drill |

---

## 1. Executive Summary

As part of NusaID Phase 9 Reliability Engineering (`PRD Section 26, Scenario A`), an intentional failure injection drill was conducted to test the system's behavior when the upstream OCR vision provider (e.g., Google Gemini 2.0 Flash / Vision AI) experiences high latency or timeout conditions (`context.DeadlineExceeded`), as well as upstream provider 500 crashes.

The drill verified that:
1. NusaID immediately halts upstream processing when context deadlines expire and returns an explicit **HTTP 504 Gateway Timeout** with standard envelope error code `ocr_failed`.
2. Provider crashes (HTTP 500 / unparseable responses) are gracefully translated to **HTTP 502 Bad Gateway** with `ocr_failed`.
3. Upstream API tokens, credentials, and sensitive provider endpoint query parameters are **strictly stripped and never leaked** into the client response payload or application logs.

---

## 2. Customer Impact

- Client applications making requests during an upstream provider outage received structured, predictable JSON error responses with HTTP 504/502 instead of socket hangs or dropped connections.
- Client SDKs and consumers were able to recognize `code: "ocr_failed"` and activate retry policies with exponential backoff.
- Zero plaintext API keys or vendor secrets were exposed.

---

## 3. Incident Timeline

| Time (UTC) | Event |
|---|---|
| `01:27:01` | Failure drill script injected `context.DeadlineExceeded` into the OCR engine adapter. |
| `01:27:02` | Client request dispatched to `POST /api/v1/ocr/ktp` with synthetic KTP image fixture. |
| `01:27:02` | NusaID OCR handler detected context deadline expiration, aborted processing, and generated OpenTelemetry error span `ocr.engine_extract`. |
| `01:27:02` | Client received **HTTP 504 Gateway Timeout** (`code: ocr_failed`, message: `"OCR engine processing timeout"`). |
| `01:27:03` | Injected upstream provider 500 error (`ocr.ErrOCRFailed`). |
| `01:27:03` | Client received **HTTP 502 Bad Gateway** (`code: ocr_failed`). |
| `01:27:04` | Verified zero secret/credential bytes in response bodies and application log streams. |

---

## 4. Root Cause Analysis (5 Whys)

1. **Why did the OCR request fail?**  
   The vision model API provider was unreachable or failed to respond within the configured timeout envelope (20 seconds).
2. **Why was the failure contained?**  
   The Go HTTP server attaches a bounded `context.WithTimeout` to incoming requests, and the `OCREngine` interface propagates `ctx context.Context` into HTTP transport calls.
3. **Why did the client not experience a hanging connection?**  
   The handler checks `errors.Is(err, context.DeadlineExceeded)` and immediately writes a 504 response without waiting for socket drops.
4. **Why were upstream API keys protected?**  
   Error responses use standardized error codes (`response.CodeOCRFailed`) and predefined human-readable strings, intentionally avoiding string interpolation of raw transport errors.
5. **Why were logs safe?**  
   The structured `telemetry.InitLogger` PII filter and error sanitizer mask credentials and prevent query parameters from being logged.

---

## 5. Verification Drill Reproduction

To reproduce this drill locally or in CI:

```bash
# Run unit drill
go test -race -v -run TestDrill_ScenarioA ./apps/api/internal/http/...

# Run unified failure drill suite
./scripts/run-failure-drills.sh
```

---

## 6. Action Items & Lessons Learned

| Action Item | Owner | Status |
|---|---|---|
| Enforce explicit HTTP 504 on `context.DeadlineExceeded` across OCR routes | Platform Eng | **Done** |
| Verify automated CI integration test for zero-leak response payloads | QA & Security | **Done** |
| Configure Grafana alert for `ocr_failed` rate exceeding 1% in 5m window | Observability | **Done** |
