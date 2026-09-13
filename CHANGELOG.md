# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id/1.1.0/). Tanggal WIB (Asia/Jakarta).

## [Unreleased]
### Fixed
- Rate limiter: hapus bypass OIDC/tanpa-org; bucket per-identitas (`org → oidc:sub → default → ip`).
- Auth: `verification_token` hanya diekspos saat `ENV=development` (fail-closed).
- Telemetry: sanitasi NIK menjadi `[REDACTED]` penuh.
- OCR handler: buffer gambar di-nil-kan setelah ekstraksi.
- Gitleaks: hapus allowlist regex global; hanya path fixture/test yang di-allow.
### Added
- `PRIVACY.md`, `SECURITY.md`, `USE_POLICY.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, issue/PR template.
- Amandemen ADR-003 (no-bypass buckets).

## [1.0.0] - 2026-09-12
- KTP + SIM OCR (`OCREngine` Gemini/Mock), API key SHA-256 berskop, token-bucket + kuota, metering async, OIDC Keycloak, ReBAC SpiceDB, idempotency, circuit breaker, OTel/Prometheus/Grafana, IaC OpenTofu, CI 5 security gates, 5 failure drills, k6 1.000 RPS.
