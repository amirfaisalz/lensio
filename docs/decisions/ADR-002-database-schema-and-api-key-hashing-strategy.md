# ADR-002: Database Schema and API Key Hashing Strategy

- **Status**: Accepted
- **Date**: 2026-09-12
- **Deciders**: Lensio Engineering Team
- **Technical Context**: `PRD Section 7`, `PRD Section 8`, `PRD Section 21`, `AGENTS.md` (Security Non-Negotiable #1)

---

## 1. Context and Problem Statement

Lensio exposes a multi-tenant B2B REST API. Tenants (organizations) authenticate via bearer API keys passed in the `Authorization: Bearer <token>` header. These keys grant programmatic access to sensitive OCR operations and usage records.

Storing API keys in plaintext or using reversible encryption creates severe security risks: if the database or backups are compromised, all client credentials would be immediately exposed. Furthermore, the database schema must balance tenant isolation, high-speed authorization lookups (O(1)), auditability of administrative actions, and non-blocking usage metering without race conditions or excessive table locking.

---

## 2. Decision Drivers

- **Zero Plaintext Secrets**: Raw API keys must never exist in persistent storage or log files.
- **Lookup Performance**: The API key authentication middleware runs on every single request. Key verification must execute with $O(1)$ indexed lookup time and negligible overhead (<2ms).
- **Tenant Isolation**: All resources (`api_keys`, `ocr_requests`, `usage_records`, `audit_logs`) must be strictly partitioned by `org_id`.
- **Revocation and Scopes**: Support granular scopes (`ocr:read`, `ocr:write`, `usage:read`), instant key revocation, expiration dates, and environment tagging (`live` vs `test`).
- **Auditability**: Track who created, modified, or revoked keys.

---

## 3. Considered Options

### Option 1: Reversible Encryption (AES-256-GCM) in Database
- *Pros*: Allows displaying existing keys to developers in the dashboard if requested.
- *Cons*: Severe security hazard. If the decryption key in Azure Key Vault or environment is exposed, all API keys are compromised. Violates industry best practices (GitHub, Stripe, AWS model).

### Option 2: Slow Hashing (Bcrypt / Argon2id)
- *Pros*: Extremely resilient against offline brute-force attacks on low-entropy passwords.
- *Cons*: Computationally expensive by design (takes 50–100ms per verification). Since API keys are high-entropy cryptographic strings (256-bit entropy) and must be verified on every API call, slow hashing creates an unacceptable latency tax and vulnerability to CPU-exhaustion DoS attacks.

### Option 3: Cryptographic Prefix + One-Way SHA-256 Hashing (Selected)
- *Pros*:
  - High-entropy tokens (32 bytes / 256 bits of CSPRNG randomness) have sufficient entropy to make SHA-256 rainbow tables and brute-force attacks computationally impossible.
  - SHA-256 hashing executes in microseconds ($O(1)$ time complexity), keeping authentication latency under 1ms.
  - Prefix routing (`lensio_live_` / `lensio_test_`) allows instant environment detection and credential leak scanning (via Gitleaks/TruffleHog regex).
  - Storing a truncated prefix (e.g., `lensio_live_abc...`) allows developers to identify keys in the dashboard without exposing the secret.
- *Cons*: If a developer loses their plaintext key, it cannot be retrieved; they must generate a new one and revoke the old one. (This is standard industry practice).

---

## 4. Decision Outcome

**Chosen Option**: **Option 3 (CSPRNG Token + Truncated Prefix + SHA-256 Hashing)**

### Implementation Details:
1. **Key Generation Format**:
   ```text
   prefix + environment + "_" + 32_random_bytes_hex
   Example: lensio_live_9f8a3c2e1b4d5e6f7a8b9c0d1e2f3a4b...
   ```
2. **Persistence Schema (`api_keys` table)**:
   - `id`: UUID (Primary Key)
   - `org_id`: UUID (Foreign Key -> `organizations.id`)
   - `name`: Human-readable label (e.g., "Production Backend")
   - `key_hash`: `VARCHAR(64)` storing the hex-encoded SHA-256 digest (Indexed `UNIQUE`)
   - `prefix`: `VARCHAR(16)` storing the identifiable prefix (e.g., `lensio_live_9f8a...`)
   - `scopes`: `TEXT[]` array containing permissions (`ocr:read`, `ocr:write`, `usage:read`)
   - `environment`: `VARCHAR(16)` (`production`, `staging`, `development`)
   - `last_used_at`: Timestamp updated asynchronously on access
   - `expires_at`: Optional timestamp for automated expiration
   - `revoked_at`: Nullable timestamp; non-null immediately denies authorization
   - `created_at`: Audit timestamp
3. **Verification Flow**:
   ```text
   Client Request -> Extract Bearer Token
                  -> Compute SHA-256(token)
                  -> SELECT * FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL
                  -> Verify scopes -> Attach APIKey context
   ```
4. **Relational Tenant Schema**:
   - `organizations` & `users`: Hierarchical tenancy with RBAC.
   - `plans`: Static lookup table defining `monthly_quota` and `rate_limit_per_minute`.
   - `ocr_requests`: Operational metadata tracking (status, confidence, latency) without storing raw PII or images.
   - `usage_records`: High-throughput time-series table for billing analytics.
   - `audit_logs`: Append-only security ledger recording who performed administrative actions.

---

## 5. Consequences

### Positive
- **Guaranteed Secret Protection**: A full database breach will not leak plaintext API keys.
- **High Throughput**: Indexed SHA-256 lookup executes in <1ms without locking tables.
- **Fast Revocation**: Revoking a key requires setting `revoked_at = NOW()`, instantly terminating access across all container instances.
- **Automated Secret Leak Detection**: Well-defined prefixes enable automated scanning tools (Gitleaks, GitHub Secret Scanning) to detect accidentally committed tokens.

### Negative / Trade-offs
- One-time viewing requires the frontend dashboard to strictly warn developers to copy their key immediately upon creation.

---

## 6. References
- `AGENTS.md` - Security & Privacy Non-Negotiables
- `PRD Section 7` - API Key Model
- `PRD Section 8` - Scopes and Permissions
- `PRD Section 21` - Database Schema
