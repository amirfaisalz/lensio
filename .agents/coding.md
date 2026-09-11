# Coding Standards

> Idiomatic Go and TypeScript conventions, Ponytail minimalism, Big O algorithmic efficiency, and quality guardrails for Lensio.

---

## 1. Ponytail Minimalist Engineering (YAGNI First)

Always apply the **Ponytail** mindset:
1. **The best code is the code never written**: If a feature, helper, or abstraction is speculative, do not build it.
2. **Climb the Ladder**:
   - *Rung 1: Does this need to exist at all?* Skip speculative needs.
   - *Rung 2: Already in this codebase?* Reuse existing types/helpers.
   - *Rung 3: Standard library does it?* Use Go stdlib (`net/http`, `crypto/sha256`, `log/slog`, `time`).
   - *Rung 4: Native platform feature?* Database unique constraints over application locks.
   - *Rung 5: Shortest working diff wins.*
3. **No Unrequested Abstractions**: No interfaces with only one implementation (except `OCREngine` mandated by spec), no premature factories, no scaffolding for "future phases".

---

## 2. Algorithmic Efficiency & Big O Benchmarking

Every algorithm and processing routine must be engineered for performance and scalability:
- **Time Complexity Benchmarks**:
  - Target $O(1)$ for lookups (hash maps, map indices) and $O(n)$ for single-pass stream processing (regex parsing, image decoding).
  - Strictly prohibit nested quadratic loops ($O(n^2)$) on hot API paths.
- **Space Complexity Benchmarks**:
  - Target $O(1)$ auxiliary space wherever possible.
  - Avoid unnecessary allocations in hot paths: reuse buffers (`sync.Pool` if justified by profiling), avoid redundant copies of image slices.
- **Microbenchmarking**:
  - For validation, hashing, or parsing logic, provide benchmark functions (`BenchmarkNIKValidation`, `BenchmarkKeyHash`) in Go test files (`*_test.go`).

---

## 3. Go Backend Conventions (`apps/api`)

### Error Handling & Propagation
- Always check errors explicitly. Never ignore errors with `_ = fn()`.
- Wrap errors with informative context using `%w`:
  ```go
  if err := store.SaveKey(ctx, key); err != nil {
      return fmt.Errorf("saving api key: %w", err)
  }
  ```
- **Zero Panic Rule**: Never call `panic()` in HTTP request handlers, middleware, or background jobs. Return domain error types.

### Standardized Error Envelope
Every non-2xx HTTP response must adhere to the standard error model:

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "API rate limit exceeded",
    "request_id": "req_01JABC123"
  }
}
```

Use only approved error codes:
- `invalid_request`: Malformed body, missing form fields, invalid JSON.
- `invalid_api_key`: Missing or unrecognized API key in `Authorization` header.
- `insufficient_scope`: Key does not possess the required scope (e.g. `ocr:write`).
- `rate_limit_exceeded`: Rate limit threshold exceeded (HTTP 429).
- `quota_exceeded`: Monthly plan quota exhausted (HTTP 429).
- `invalid_document`: Unreadable, corrupted, or unsupported image format (HTTP 400).
- `unsupported_document`: Valid image but not an Indonesian KTP (HTTP 422).
- `ocr_failed`: Upstream OCR engine failure or timeout (HTTP 502 / 504).
- `low_confidence`: OCR extracted text but confidence score fell below acceptable threshold.
- `internal_error`: Unhandled server exception (HTTP 500).

### Structured Logging (`log/slog`)
- Use the standard library `log/slog` with JSON handler.
- Always include `request_id` in logs within request contexts:
  ```go
  logger.InfoContext(ctx, "ocr request processed",
      slog.String("request_id", reqID),
      slog.Int("latency_ms", latency),
      slog.Int("status", http.StatusOK),
  )
  ```
- **CRITICAL**: Never log PII (NIK, names, birth dates, full addresses) or raw file buffers.

### Zero Lint Errors
- Code must pass `golangci-lint run ./...` cleanly with zero warnings or suppressed rules.

---

## 4. Frontend Conventions (`apps/dashboard`)

- **Language**: TypeScript with strict mode enabled (`noImplicitAny: true`, `strictNullChecks: true`).
- **Zero Type Errors**: Prohibit `any` types; define strict interfaces for all API payloads.
- **Framework**: React 18+ with functional components and hooks.
- **Styling**: Tailwind CSS utility classes.
- **API Types**: Generate TypeScript interfaces directly from `openapi/openapi.yaml`. Do not hand-write duplicate DTO models.
- **Component Design**: Co-locate component logic, types, and presentation in focused, self-contained files.
