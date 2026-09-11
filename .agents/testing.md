# Testing Strategy

> Rules for automated testing, test data safety, strict test integrity, and 100% coverage target in NusaID.

---

## 1. Core Testing Mandates

### 100% Test Coverage Target
Every feature, service layer, algorithm, and validation logic must strive for **100% test coverage**:
- NIK format and checksum validation
- Date parsing and calendar sanity
- Key generation, SHA-256 hashing, and scope RBAC checks
- Rate limiting window logic and header emission
- Quota deduction and exhaustion threshold checks
- OCR result normalization and confidence calculation

### Test Integrity & Zero Gaming
- **Never cheat tests**: Any test that uses tautological assertions (`assert true == true`), empty checks, or mocks away the actual logic being tested is strictly prohibited.
- **True Red-Green Verification**: Every test must genuinely prove:
  1. **Failure states (Negative testing)**: Invalid NIKs, wrong image types, corrupted files, expired/revoked API keys, exhausted quotas, and out-of-scope requests MUST fail with exact expected error codes and HTTP statuses.
  2. **Pass states (Positive testing)**: Valid inputs, exact extracted fields, and correct state transitions MUST succeed with exact assertions.
- **Race Condition Testing**: All Go test executions MUST run with `-race`:
  ```bash
  go test -race -cover -v ./...
  ```

---

## 2. Test Data Safety: Synthetic Only

> [!CAUTION]
> **STRICT PII PROHIBITION**: Real Indonesian identity cards (KTP) or real personal citizen data must NEVER be committed to this repository or used in test suites.

- Always use generated **synthetic test KTP images** or heavily redacted sample fixtures.
- Test NIKs should use fictitious area codes (e.g., `3171000000000001`) and standard dummy names (e.g., `BUDI SANTOSO`, `JANE DOE`).
- All test fixtures belong under `tests/fixtures/synthetic/`.

---

## 3. The Testing Pyramid

```text
       ▲
      / \     E2E Tests (Playwright)
     /   \    - Full browser journeys: Login -> Key -> OCR -> Dashboard -> Revoke
    /─────\
   /       \   Integration Tests (Go)
  /         \  - API <-> Postgres migrations, Quota increments, Rate limiter stores
 /───────────\
/             \ Unit Tests & Benchmarks (Go & TypeScript)
               - NIK validation, OCR parsing, key hashing, scope RBAC, handlers
```

### Mocking Guidelines
- In unit tests and CI integration runs, use `MockOCREngine` located in `services/ocr/providers/mock.go`.
- Avoid calling external third-party APIs (such as live Gemini or cloud providers) in standard test suites. Tests must be fast, deterministic, and runnable offline.

---

## 4. End-to-End Testing (Playwright)

Located in `tests/e2e/`. Playwright tests verify the critical user journeys:
1. Developer logs in to the developer portal.
2. Developer generates a new API key (`fg_live_...`) with `ocr:write` scope and copies the key.
3. API client submits a synthetic KTP image to `POST /api/v1/ocr/ktp`.
4. API responds with structured JSON containing expected synthetic NIK and nama.
5. Developer navigates to Overview/Usage in the portal and confirms request count and quota gauge increment.
6. Developer revokes the API key and confirms subsequent API requests return `401 Unauthorized`.
