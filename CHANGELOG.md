# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id/1.1.0/). Tanggal WIB (Asia/Jakarta).

## [Unreleased]

## [1.1.0] - 2026-09-13
### Added
- Passport, NPWP, KK & Invoice OCR: 6 tipe dokumen live (Gemini + Mock engine, fixture sintetis, kontrak OpenAPI, tab Docs & Playground).
- Playground cookie-first ala OpenAI (tanpa API key), pratinjau gambar + zoom inline, header perbandingan + salin JSON.
- Brand Lensio: set favicon, logo sidebar/login, logo header README.
- Metrik P95 terpisah: SLA platform vs latensi upstream AI (tanpa SLA).
- Middleware `SessionOrg`: atribusi org untuk pemanggil cookie di endpoint OCR.
### Fixed
- Rate limiter: bypass `/api/v1/account/*`; cache + retry dashboard anti-429; callout first-key hanya jika org nol key.
- Deploy: fail-fast tanpa skip diam-diam; otomatis hanya ke staging, produksi manual-only.
- CI: `AuthContext` useCallback agar `biome check` hijau.
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
