# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id/1.1.0/). Tanggal WIB (Asia/Jakarta).

## [Unreleased]
### Fixed
- Kuota: hitung semua `POST /api/v1/ocr/*` (sebelumnya hanya KTP) di `GetMonthlyOCRCount`/`GetUsageSummary`.
- Idempotency: respons 429 tidak di-cache; lock di-release agar retry pasca-reset tidak replay 429 basi.
- Auth: tolak start prod/staging tanpa `SESSION_SECRET`/`DATABASE_URL`; cookie `Secure` paksa di production; guard cross-org `org_id` tanpa SpiceDB; scope sesi cookie di-allowlist.
- Rate limit: hapus override `?org_id` OIDC; login/register/verify-email masuk bucket IP (anti brute-force).
- Hardening: cap hash idempotency 6MB, fallback UUID/request-ID unik, validasi kontainer WebP, Enforcer fail-closed, error registrasi/verifikasi email dipropagasikan ke UI.
### Changed
- OCR handler: 6 handler (`ocr.go` 1941→±640 baris) dilebur ke satu pipeline generik + config per dokumen; perilaku byte-identik.
### Added
- `docs/audits/2026-09-14-implementation-audit.md`: temuan + status perbaikan audit security/wiring.

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
