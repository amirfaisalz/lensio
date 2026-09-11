# Lensio Demo Video & Presentation Walkthrough Guide

> Production walkthrough script and narration cues for recording a 3–5 minute video demo (`PRD Section 36 & 37`).  
> Demonstrates the complete lifecycle: Authentication, OCR Extraction, Telemetry, Rate Limiting, and Automated Rollback.

---

## 1. Demo Metadata

- **Title**: Lensio – Production-Grade Indonesian KTP OCR Platform Engineering Demo
- **Target Duration**: 3 to 5 minutes
- **Required Windows / Layout**:
  - **Left Window**: Terminal (Split: Client requests on top, live server logs on bottom).
  - **Right Window**: Lensio Developer Dashboard (`http://localhost:5173` or staging URL).

---

## 2. Walkthrough Flow Summary

```text
[0:00 - 0:45] Scene 1: Problem, Mission & Developer Portal Login
      ↓
[0:45 - 1:15] Scene 2: Scoped API Key Creation & Cryptographic Hashing
      ↓
[1:15 - 2:00] Scene 3: Synchronous KTP OCR Request with Synthetic Fixture
      ↓
[2:00 - 2:45] Scene 4: Real-Time Usage Metering & Quota Progress Update
      ↓
[2:45 - 3:30] Scene 5: Tiered Rate Limiting in Action (HTTP 429 Simulation)
      ↓
[3:30 - 4:30] Scene 6: Deployment Promotion, Failure Drill & Instant Rollback (<60s)
      ↓
[4:30 - 5:00] Scene 7: Wrap-up & Platform Engineering Portfolio Highlights
```

---

## 3. Scene-by-Scene Script & Execution

### Scene 1: Introduction & Developer Portal (0:00 – 0:45)
- **Visual**: Show Developer Dashboard Overview page (`http://localhost:5173`).
- **Narration**:
  > *"Lensio is not just an OCR script—it is an end-to-end API product. Our mission is to provide an affordable, developer-first Indonesian KTP OCR service built with production platform engineering rigor. In this demo, we'll walk through the complete lifecycle: authenticating with cryptographic API keys, extracting structured KTP data from synthetic fixtures, monitoring non-blocking usage metrics, enforcing rate limits, and performing a live sub-60-second revision rollback."*

---

### Scene 2: API Key Lifecycle & Scopes (0:45 – 1:15)
- **Visual**: Navigate to **API Keys** tab in the dashboard or run CLI command in terminal.
- **Terminal Action**:
  ```bash
  # Generate a new scoped API key for VeriForm onboarding
  curl -s -X POST http://localhost:8080/api/v1/auth/api-keys \
    -H "Content-Type: application/json" \
    -d '{
      "name": "VeriForm Production Client",
      "scopes": ["ocr:write", "ocr:read", "usage:read"],
      "environment": "production"
    }' | jq .
  ```
- **Expected Output**:
  ```json
  {
    "id": "key_01JABC12345",
    "name": "VeriForm Production Client",
    "token": "lensio_live_9f8a3c2e1b4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d",
    "prefix": "lensio_live_9f8a",
    "scopes": ["ocr:write", "ocr:read", "usage:read"]
  }
  ```
- **Narration**:
  > *"We issue a scoped API key. Notice that the plaintext token is displayed exactly once. In our PostgreSQL database, only the SHA-256 hash is persisted. The prefix allows developers to recognize the key in the dashboard while keeping secret data strictly unrecoverable if storage is breached."*
- **Set Variable**:
  ```bash
  export API_KEY="lensio_live_9f8a3c2e1b4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d"
  ```

---

### Scene 3: KTP OCR Extraction with Synthetic Fixture (1:15 – 2:00)
- **Visual**: Split screen showing terminal and server logs.
- **Terminal Action**:
  ```bash
  # Send synthetic KTP image to the OCR endpoint
  curl -s -X POST http://localhost:8080/api/v1/ocr/ktp \
    -H "Authorization: Bearer $API_KEY" \
    -F "document=@tests/fixtures/synthetic/valid_ktp.jpg" | jq .
  ```
- **Expected Output**:
  ```json
  {
    "id": "ocr_01JABC1234567890",
    "status": "completed",
    "document_type": "ktp",
    "confidence": 0.98,
    "data": {
      "nik": "3171012345670001",
      "nama": "BUDI SANTOSO",
      "tempat_lahir": "JAKARTA",
      "tanggal_lahir": "1992-08-17",
      "jenis_kelamin": "LAKI-LAKI",
      "alamat": "JL. MERDEKA NO. 45",
      "kewarganegaraan": "WNI"
    },
    "processing": {
      "latency_ms": 1420
    }
  }
  ```
- **Server Logs Observation**:
  ```json
  {"level":"INFO","msg":"ocr_completed","request_id":"req_01JABC","confidence":0.98,"latency_ms":1420}
  ```
- **Narration**:
  > *"The API validates the MIME type and magic bytes in pure Go, streams the buffer to our pluggable OCREngine, and deterministically validates the 16-digit NIK and regional codes. In under 1.5 seconds, we receive structured JSON. Look at the structured log below: zero PII. No NIK, no names, and no uploaded images are written to disk or logs, complying with Indonesian PDP privacy laws."*

---

### Scene 4: Real-Time Usage & Quota Progress (2:00 – 2:45)
- **Visual**: Refresh dashboard or view terminal usage endpoint.
- **Terminal Action**:
  ```bash
  curl -s http://localhost:8080/api/v1/usage \
    -H "Authorization: Bearer $API_KEY" | jq .
  ```
- **Expected Output**:
  ```json
  {
    "total_requests": 1,
    "successful_requests": 1,
    "error_requests": 0,
    "quota_limit": 100,
    "quota_remaining": 99,
    "plan": "free"
  }
  ```
- **Narration**:
  > *"Notice how the request counter instantly updated without adding latency to the client response. That's because usage metering is dispatched asynchronously across buffered Go worker channels."*

---

### Scene 5: Rate Limiting & Protection in Action (2:45 – 3:30)
- **Visual**: Terminal executing rapid burst requests exceeding the Free tier limit (10 req/min).
- **Terminal Action**:
  ```bash
  # Fire a burst of 12 rapid requests
  for i in {1..12}; do
    curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/api/v1/ocr/ktp \
      -H "Authorization: Bearer $API_KEY" \
      -F "document=@tests/fixtures/synthetic/valid_ktp.jpg"
  done
  ```
- **Expected Output**:
  ```text
  200
  200
  ...
  429
  429
  ```
- **Inspect Response Headers**:
  ```bash
  curl -i -X POST http://localhost:8080/api/v1/ocr/ktp \
    -H "Authorization: Bearer $API_KEY" \
    -F "document=@tests/fixtures/synthetic/valid_ktp.jpg"
  ```
- **Expected Headers**:
  ```http
  HTTP/1.1 429 Too Many Requests
  X-RateLimit-Limit: 10
  X-RateLimit-Remaining: 0
  Retry-After: 42
  Content-Type: application/json

  {
    "error": {
      "code": "rate_limit_exceeded",
      "message": "API rate limit exceeded. Please retry after 42 seconds.",
      "request_id": "req_01JXYZ..."
    }
  }
  ```
- **Narration**:
  > *"When the client exceeds their plan's burst allowance, the in-memory token bucket rate limiter intercepts the call before image parsing occurs. It returns HTTP 429 with standard headers: X-RateLimit-Remaining 0 and Retry-After."*

---

### Scene 6: Deployment, Failure Simulation & Instant Rollback (3:30 – 4:30)
- **Visual**: Show GitHub Actions workflow or run local rollback drill script.
- **Terminal Action**:
  ```bash
  # Execute automated failure drill D (Production Regression Drill)
  go test -race -v -run TestIntegration_Drill_ScenarioD ./tests/integration/...
  ```
- **Show Azure CLI Rollback Command**:
  ```bash
  az containerapp revision set-traffic \
    --name ca-api-lensio-production \
    --resource-group rg-lensio-production \
    --revision-weight ca-api-lensio-prod--1-4-0=100
  ```
- **Narration**:
  > *"In production, revisions are immutable. If an error spike exceeds our 1.0% error budget, our automated rollback workflow shifts 100% of ingress traffic back to the prior stable container revision. Because ACA switches traffic at the Envoy proxy level without re-pulling images, recovery is achieved in under 30 seconds."*

---

### Scene 7: Demo Consumer & Portfolio Conclusion (4:30 – 5:00)
- **Visual**: Run independent demo consumer script.
- **Terminal Action**:
  ```bash
  ./scripts/run-demo-consumers.sh
  ```
- **Narration**:
  > *"To prove Lensio is a true API product rather than a monolithic demo, independent external consumers like VeriForm and RentEase consume the API for identity verification. Lensio demonstrates complete platform engineering ownership: Go, PostgreSQL, React, OpenTelemetry, Azure Container Apps, OpenTofu, and automated security gates."*

---

## 4. Recording Checklist

- [ ] Clear any real personal data or credentials from terminal history.
- [ ] Verify test database is seeded with default plans (`free`, `starter`, `pro`).
- [ ] Test synthetic image fixture exists at `tests/fixtures/synthetic/valid_ktp.jpg`.
- [ ] Set terminal font size to 14–16pt with high-contrast color scheme.
- [ ] Record in 1080p (1920x1080) at 30 or 60 fps.
