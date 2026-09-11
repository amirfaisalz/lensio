# NusaID Security & Privacy Architecture

> Comprehensive security specification, threat model, compliance posture, and automated security controls.  
> Governed by `AGENTS.md` (Mandatory Rules 4 & 5) and `PRD Section 15`.

---

## 1. Security Philosophy: Defense in Depth

NusaID processes Indonesian identity documents (KTP). Because national identity cards contain sensitive citizen identity markers (NIK, full names, addresses, photos), security and privacy are treated as core platform architectural constraints rather than afterthoughts.

Our posture is built on five non-negotiable pillars:
1. **Zero Plaintext Secrets**: Cryptographic hashing of API tokens.
2. **Strict Data Minimization**: Ephemeral image handling; zero permanent storage of raw documents.
3. **Zero PII in Application Logs**: Strict scrubbing of all citizen personal data.
4. **Automated Continuous Verification**: Pre-commit hooks, SAST, dependency scanning, secret audits, and container CVE gates.
5. **Auditable Administrative State**: Immutable audit trail for all key generation and revocation events.

---

## 2. Threat Modeling: STRIDE & OWASP API Security Top 10

| Category | Threat Scenario | Impact | NusaID Mitigation Control |
|---|---|---|---|
| **Spoofing** | Adversary attempts to forge or brute-force API keys | Unauthorized API access, identity data extraction | High-entropy CSPRNG tokens (256-bit entropy) with SHA-256 one-way hashing; brute-forcing is computationally infeasible. |
| **Tampering** | Attacker intercepts or modifies image/JSON payloads in transit | Identity fraud, payload corruption | Mandatory TLS 1.3 edge-to-container; Cloudflare WAF inspection; strict deterministic NIK structure validation. |
| **Repudiation** | Client disputes API invocations or key revocation | Billing or security disputes | Append-only `audit_logs` table recording `actor_id`, `target_resource`, and timestamp for all administrative events. |
| **Information Disclosure** | Database dump or log aggregator breach exposes KTP images or citizen PII | Catastrophic privacy violation & PDP Law penalty | **Zero raw image storage** (ephemeral RAM buffers only). Strict PII log redaction (zero NIK, names, or addresses logged). |
| **Denial of Service** | Volumetric traffic spike or slow HTTP multipart attack | API degradation, upstream quota exhaustion | Multi-tier rate limiting (in-memory token bucket) + Cloudflare edge DDoS mitigation + 5MB multipart upload size limit. |
| **Elevation of Privilege** | Read-only API key attempts to trigger administrative or OCR operations | Unauthorized tenant actions | Granular scope enforcement (`ocr:read`, `ocr:write`, `usage:read`) validated at HTTP middleware layer. |

---

## 3. Cryptographic Key Management & Hashing Strategy

API keys authenticate programmatic consumers. To eliminate the danger of key compromise via database exfiltration, plaintext keys are **never stored**.

```text
[Key Generation]
  1. Generate 32 cryptographically secure random bytes (crypto/rand).
  2. Hex-encode token: 9f8a3c2e...
  3. Prepend prefix: nusa_live_9f8a3c2e... (Plaintext returned to user ONCE)
  4. Compute SHA-256 hash: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
  5. Store only: { key_hash, prefix: "nusa_live_9f8a", scopes: ["ocr:write"], org_id }

[Key Verification]
  Incoming Header: Authorization: Bearer nusa_live_9f8a3c2e...
  1. Hash received token: digest = SHA-256(received_token)
  2. SQL Lookup: SELECT * FROM api_keys WHERE key_hash = digest AND revoked_at IS NULL
  3. Verify expiration and assert requested scopes.
```

- **One-Way Hash**: Impossible to reverse the database digest to recover the plaintext key.
- **Prefix Identification**: Truncated prefix (`prefix`) enables identification in the Developer Dashboard without revealing the secret.
- **Scannable Secret Pattern**: Prefixes (`nusa_live_` and `nusa_test_`) allow automated regex scanners (Gitleaks, GitHub Secret Scanning) to prevent accidental repository commits.

---

## 4. Indonesian PDP Law Compliance & Data Minimization

Under Indonesia's Personal Data Protection Law (**UU No. 27 Tahun 2022 tentang Pelindungan Data Pribadi**), processing personal data requires strict compliance with principles of **purpose limitation**, **data minimization**, and **storage limitation**:

### 1. Ephemeral In-Memory Processing
- Uploaded KTP multipart payloads are read directly into an in-memory byte buffer (`[]byte`).
- The buffer is validated (MIME type, magic byte inspection for JPEG/PNG/WebP, max 5MB).
- The buffer is streamed to the vision inference provider (`OCREngine`).
- Once extraction and validation complete, the Go memory buffer is released.
- **Under no circumstances is the image saved to local disk, temp files, or cloud blob storage**.

### 2. Zero PII in Telemetry and Logs
- Application loggers (`log/slog`) are strictly configured to log operational telemetry only:
  - Allowed: `request_id`, `trace_id`, `span_id`, `org_id`, `api_key_id`, `latency_ms`, `status_code`, `confidence`, `document_type`.
  - Strictly Prohibited: `nik`, `nama`, `tempat_lahir`, `tanggal_lahir`, `alamat`, `rt_rw`, raw images, or OCR response bodies.

### 3. Deterministic Validation First
Extracted identity fields are never trusted blindly from vision models:
- **NIK Validation**:
  - Must match `^[0-9]{16}$` (exactly 16 digits).
  - Province code (first 2 digits) must match official Indonesian administrative codes (e.g., `31` for DKI Jakarta, `32` for Jawa Barat).
  - Date encoding check: Digits 7–8 represent day of birth (offset by +40 for female citizens), digits 9–10 represent month (01–12), digits 11–12 represent two-digit birth year.
- **Date Validation**: Dates are parsed deterministically against calendar bounds (`YYYY-MM-DD`).

---

## 5. Automated CI/CD Security Gates

Every Git commit and Pull Request must pass automated security scans before merging or deploying:

```text
Commit / Pull Request
         │
         ├──► 1. Secret Scanning (Gitleaks / TruffleHog)
         │       - Scans full commit history for API tokens, passwords, and private keys.
         │
         ├──► 2. Go Vulnerability Check (govulncheck)
         │       - Audits Go module call graphs against the official Go vulnerability database.
         │
         ├──► 3. SAST Static Application Security Testing (gosec)
         │       - Scans Go AST for SQL injection, insecure file reads, and weak crypto.
         │
         ├──► 4. IaC Security Scanning (Trivy)
         │       - Scans OpenTofu/Terragrunt configurations for insecure network rules.
         │
         └──► 5. Container Image Vulnerability Scanning (Trivy)
                 - Scans Docker layers for CVEs with severity HIGH,CRITICAL (exit code 1 on violation).
```

---

## 6. Audit Logging & Administrative Accountability

All sensitive security and administrative operations are recorded in an append-only `audit_logs` table:
- API key generation and token issuance.
- API key revocation.
- Tenant plan modifications or quota updates.
- Organization member invitations and role changes.

Audit records contain:
- `id`: Unique UUID
- `org_id`: Tenant organization identifier
- `actor_id`: User or service that performed the action
- `action`: Specific operation (e.g. `api_key.created`, `api_key.revoked`, `plan.updated`)
- `target_resource`: Affected entity identifier (e.g. key ID or plan code)
- `metadata`: JSON object containing contextual metadata (excluding plaintext secrets)
- `created_at`: Immutable timestamp
