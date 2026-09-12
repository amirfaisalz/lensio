# Security and Privacy Policy

> Core security architecture, data minimization rules, and automated vulnerability scanning standards.

---

## 1. Data Minimization & PII Protection

Indonesian KTP documents contain sensitive Personally Identifiable Information (PII). Security and privacy are primary functional requirements, not afterthoughts.

```text
Client Upload (KTP Image)
       │
       ▼
In-Memory / Ephemeral Buffer
       │
       ▼
OCR & Field Extraction
       │
       ▼
Structured Response Emitted
       │
       ▼
[BUFFER IMMEDIATELY DESTROYED]
```

### Mandates
1. **Zero Permanent Image Storage**: Lensio does not store raw KTP images in object storage or databases.
2. **Ephemeral Memory Only**: The image byte slice is read, processed, and released for garbage collection immediately after OCR execution.
3. **No PII in Application Logs**:
   - Forbidden in logs: `nik`, `nama`, `tempat_lahir`, `tanggal_lahir`, `alamat`, `rt_rw`, base64 image data.
   - Permitted in logs: `request_id`, `organization_id`, `api_key_id` (UUID), `status_code`, `latency_ms`, `document_type`.

---

## 2. API Key Lifecycle & Storage Security

1. **Secret Generation**: Keys are generated with 32+ bytes of cryptographic entropy (`crypto/rand`) using a distinct prefix (`lensio_live_...` or `lensio_test_...`).
2. **One-Time Secret Presentation**: The full plaintext key is displayed to the developer exactly once upon creation. It is never retrievable again.
3. **Storage Rule**: Only the cryptographic hash (SHA-256) of the token is saved in PostgreSQL:
   ```text
   Stored columns: id, org_id, name, prefix (e.g. "lensio_live_3a..."), key_hash, scopes, created_at, revoked_at
   ```
4. **Instant Revocation**: Revoking an API key sets `revoked_at = NOW()`, immediately invalidating subsequent calls.

---

## 3. Input Validation & Defense in Depth

- **File Size**: Reject any upload exceeding 5MB with HTTP 400 (`invalid_request`).
- **MIME & Magic Bytes**: Inspect the initial byte stream to verify valid image headers (JPEG: `0xFF 0xD8 0xFF`, PNG: `0x89 0x50 0x4E 0x47`, WebP). Do not trust client-supplied `Content-Type` headers alone.
- **Deterministic Field Validation**: Validate NIK format (16 digits, numeric) before returning response to client.

---

## 4. Automated CI Security Gates

Every Pull Request must pass automated security audits:
- **Secret Scanning**: `gitleaks` prevents accidental credential commits.
- **Dependency Vulnerabilities**: `govulncheck ./...` for Go and `npm audit` for dashboard.
- **Static Application Security Testing (SAST)**: `gosec -quiet ./...` scans for security smells in Go.
- **Container Vulnerability Scanning**: `trivy image` scans Docker images for CVEs.
- **IaC Scanning**: `tfsec` or `checkov` validates OpenTofu code.

Any finding graded **HIGH** or **CRITICAL** fails the CI pipeline.

---

## 5. OWASP Authentication, Session & Browser Storage Standards

In alignment with **OWASP ASVS (Application Security Verification Standard)** and **OWASP Cheat Sheet Series (Session Management & HTML5 Storage)**:

1. **Secure Session Cookies (`Set-Cookie: lensio_session`)**:
   - `HttpOnly`: Strictly prevents client-side scripts from reading session credentials (`document.cookie`), mitigating Cross-Site Scripting (XSS) session hijacking.
   - `Secure`: Ensures session tokens are only transmitted over TLS/HTTPS channels (with localhost allowance for local dev).
   - `SameSite=Lax`: Defends against Cross-Site Request Forgery (CSRF) for cross-origin state-changing mutations.
   - `Path=/`: Restricts scope to application boundaries.
2. **Zero Storage Mandate for Browser Client**:
   - **Zero Secrets in Storage**: Never store JWTs, access tokens, refresh tokens, API keys, or raw passwords in `localStorage` or `sessionStorage`.
   - **Zero PII in Storage**: Never persist user identity profiles (`email`, `full_name`, citizen data, roles) in browser web storage.
   - **Pure Ephemeral In-Memory State**: Maintain active user identity in React memory (`useState`/context) only. Rehydrate identity on page load/refresh by verifying the session cookie via `GET /api/v1/auth/me`.
3. **Server-Side Session Invalidation**:
   - Explicit logout (`POST /api/v1/auth/logout`) sets `Max-Age: -1` and expires the session cookie immediately.
4. **CORS & Credential Isolation**:
   - When handling authenticated cookie sessions, `Access-Control-Allow-Credentials: true` is strictly paired with explicit/reflected origins—wildcard (`*`) origins with credentials are strictly forbidden.

