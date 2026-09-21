# Lensio AI Agent Operational Guide

> Single source of truth for AI agents (and human contributors) working on the Lensio codebase.
> **MANDATORY IN ALL SESSIONS**: All rules in this document must be strictly obeyed across every session without exception.

---

## 1. Project Identity & Purpose

**Lensio** is an affordable Indonesian Identity Document OCR API (KTP, SIM, Passport, NPWP, KK & Invoice) as a service. It is designed as a production-grade API product demonstrating end-to-end platform engineering:
- **Language & Runtime**: Go (Backend API) + React & TypeScript (Developer Dashboard)
- **Database**: PostgreSQL 16 (Relational state, migrations via `golang-migrate`)
- **Primary OCR Engine**: Pluggable `OCREngine` interface. Default Vision AI: **Google Gemini Flash** — the default model id lives in `services/ocr/providers/gemini.go` (`defaultGeminiModel`) and is overridable via `GEMINI_MODEL`; config intentionally does not duplicate it. Default Test Engine: **MockOCREngine** (deterministic fixtures).
- **Platform Infrastructure**: Azure Container Apps, Cloudflare, OpenTofu, Terragrunt, OpenTelemetry, Grafana.

### Core Engineering Principle
> **"Build the smallest real product that forces us to solve real production engineering problems."**
> Never build features merely to add tech to a README. Every piece exists because the product demands it.

---

## 2. Current Strategic Focus (Strict MVP Scope)

- **Target Documents**: **Indonesian KTP** (`POST /api/v1/ocr/ktp`), **SIM** (`POST /api/v1/ocr/sim`), **Passport** (`POST /api/v1/ocr/passport`), **NPWP** (`POST /api/v1/ocr/npwp`), **KK** (`POST /api/v1/ocr/kk`), and **Invoice** (`POST /api/v1/ocr/invoice`).
- **Target Platform**: API authentication, scoped API keys, rate limiting, quota enforcement, non-blocking usage metering, `/health` and `/ready` probes, OpenTelemetry instrumentation.
- **Future Product Lines**: Document verification, identity verification, and custom/unstructured extraction are explicitly deferred to post-MVP iterations. **Do not implement them now.**

---

## 3. Five Mandatory Agent Operating Rules (Enforced in ALL Sessions)

All AI agents must strictly follow these five non-negotiable rules on every task:

### Rule 1: Always Apply Ponytail Skills (Minimal & Simplest Solution)
- **Lazy Senior Dev Mindset**: The best code is the code never written. Always find the simplest, shortest, most minimal solution that actually works (YAGNI).
- **Climb the Ponytail Ladder**:
  1. *Does this need to exist at all?* (If speculative, skip it).
  2. *Already in this codebase?* (Reuse existing utils/types).
  3. *Standard library does it?* (Reach for Go `net/http`, `crypto/sha256`, `log/slog` before adding dependencies).
  4. *Native platform feature covers it?* (DB constraint, native HTML/CSS).
  5. *Shortest working diff wins.*
- **Rules**:
  - No unrequested abstractions: no interface with only one implementation (except `OCREngine` required by spec), no premature factories, no scaffolding "for later".
  - Deletion over addition. Boring over clever.
  - **NEVER be lazy about**: Understanding the problem, input validation at boundaries, security, error handling, or test correctness.

### Rule 2: Strict Test Integrity & 100% Coverage Target
- Every single feature and algorithm must have comprehensive tests targeting **100% coverage**.
- **Absolute Test Integrity**: NEVER game, bypass, or fake tests (e.g. no dummy `assert true == true`, no mocking away the actual logic being evaluated, no empty assertions).
- **True Red-Green Verification**: Every test must genuinely prove both the **failed** state (negative tests, malformed inputs, error branches, edge cases) and the **passed** state (valid inputs, expected outputs).
- **Race Detection Mandate**: Always run tests with race detection enabled: `go test -race -cover ./...`.
- **Synthetic Test Data Only**: NEVER use real Indonesian KTP or citizen identity data. Use synthetic fixtures in `tests/fixtures/synthetic/`.

### Rule 3: Algorithmic Efficiency & Big O Benchmarking
- All algorithms and data processing pipelines must be fast, optimal, and low-overhead.
- **Big O Notation as Benchmark**:
  - Explicitly evaluate Time Complexity (prioritize $O(1)$ lookups and $O(n)$ single-pass processing; strictly avoid accidental $O(n^2)$ quadratic loops in hot paths).
  - Explicitly evaluate Space Complexity (keep memory allocations minimal; avoid heap escapes where stack allocation suffices).
- Benchmark critical algorithms (regex extractors, NIK validators, rate limit evaluators) using Go benchmarks (`testing.B`).

### Rule 4: Zero Lint Errors, Zero Type Errors & Strict Best Practices
- **Zero Type Errors**: Strict Go compilation, strict TypeScript (`strict: true`, `noImplicitAny: true`, zero `any`).
- **Zero Lint Errors**: Must pass `golangci-lint run ./...` and frontend linters without warnings or suppressed errors.
- **Idiomatic Best Practices**:
  - Always wrap errors with context (`fmt.Errorf("...: %w", err)`).
  - **Zero Panics**: Never call `panic()` in production paths; use standardized error envelopes.
  - Structured logging with `log/slog` (JSON format, correlated with `request_id`, zero PII).

### Rule 5: Pre-Commit Git Hooks Enforced on Every Commit
- Git hooks in `.githooks/pre-commit` must be active and configured via `git config core.hooksPath .githooks`.
- **Always run and pass all 4 verification layers before committing code**:
  - **Layer 1**: Type Checking & Compilation (`go build ./...`, `tsc --noEmit`)
  - **Layer 2**: Linting & Best Practices (`golangci-lint` / `go vet`, frontend lint)
  - **Layer 3**: Strict Tests & Coverage with Race Detector (`go test -race -cover ./...`)
  - **Layer 4**: Big O, Security Guardrails & PII leak prevention
- Never bypass pre-commit hooks with `--no-verify`.
- **Push Policy**: **NEVER automatically run `git push`**. Always stop at `git commit` to conserve GitHub Actions workflow quota. Only run `git push` when explicitly requested by the user.

---

## 4. Security & Privacy Non-Negotiables

1. **Zero Plaintext Secrets**: Store only cryptographic hashes (SHA-256) of API keys in PostgreSQL. Plaintext keys are returned once upon generation and never stored.
2. **Strict Data Minimization**: Never store uploaded KTP images. Process in memory/temporary buffer and discard immediately.
3. **Zero PII in Logs**: Never write NIK, full names, addresses, or dates of birth to application logs. Correlate using `request_id` and `trace_id` only.
4. **Deterministic Validation First**: Validate NIK (16 digits, valid numeric structure) and dates deterministically. Do not rely solely on LLM output.
5. **No Leaky Abstractions**: Keep OCR providers strictly behind the `OCREngine` interface. The HTTP API layer must never know provider details.
6. **OWASP Best Practice for Session Security**:
   - Web browser sessions must strictly use **Secure, HttpOnly, SameSite=Lax** cookies (`lensio_session`).
   - **Zero Tokens & Zero PII in Browser Storage**: Never store JWT tokens, secrets, or user identity profiles (`email`, `full_name`, roles) in `localStorage` or `sessionStorage` (preventing XSS token theft and unauthorized disk inspection).
   - Frontend user profile state must remain **100% ephemeral in-memory** (React state), verified on initial app load and refresh via `GET /api/v1/auth/me`.
   - Logouts must explicitly invalidate cookies server-side with `Max-Age: -1`.


---

## 5. Context Pointers (`.agents/`)

Read these files based on the nature of your current task:

| Topic | File Pointer | When to Read |
|---|---|---|
| **Architecture & Structure** | [`.agents/architecture.md`](.agents/architecture.md) | Adding modules, altering database schemas, introducing new services or interfaces. |
| **Coding Standards** | [`.agents/coding.md`](.agents/coding.md) | Writing or refactoring Go backend code, error envelopes, or React dashboard code. |
| **Testing & Quality** | [`.agents/testing.md`](.agents/testing.md) | Writing unit tests, integration tests, mocks, or Playwright E2E tests. |
| **Security & Privacy** | [`.agents/security.md`](.agents/security.md) | Handling authentication, authorization, API keys, PII sanitization, or input validation. |
| **Deployment & Ops** | [`.agents/deployment.md`](.agents/deployment.md) | Working on Dockerfiles, OpenTofu, CI/CD workflows, health probes, or rollback procedures. |

---

## 6. Development Phase Tracking

All phases and granular deliverables are tracked in:
👉 **[`DEVELOPMENT_TRACKING.md`](DEVELOPMENT_TRACKING.md)**

Update the checklist checkmarks `[x]` as each item's acceptance criteria are verified.
