# Changelog

Format mengikuti [Keep a Changelog](https://keepachangelog.com/id/1.1.0/). Tanggal WIB (Asia/Jakarta).

## [Unreleased]
### Added
- **Validator conformance terhadap spesifikasi** (ICAO 9303, Permendagri 109/2019, PMK 112/2022): spesimen MRZ resmi + 4 test vector check-digit, tiap check digit dibuktikan load-bearing, Luhn NPWP dihitung ulang independen. Menutup 5 celah: check digit personal number MRZ dibuang (`_ =`), karakter sex tidak divalidasi, kode kabupaten/kecamatan `00` diterima, tarif PPN tidak pernah diperiksa (0,5% lolos), diskon tak berbatas.
- **Pengiriman email transaksional** (`net/smtp`, tanpa dependensi baru) + **reset password**: sebelumnya token verifikasi dibuat lalu tidak pernah dikirim, sehingga signup mandiri mustahil selesai di produksi; dan tidak ada jalur pemulihan password sama sekali.
- **Rate limit cluster-wide** lewat counter Postgres (`rate_limit_counters`), menggantikan bucket per-proses yang membolehkan burst hingga Nx limit paket pada N replika.
- **Keanggotaan organisasi sungguhan** (`organization_members`): satu user kini bisa di banyak organisasi dengan peran per-organisasi. `GET /api/v1/account/organizations` menjadikan server sumber kebenaran; dashboard berhenti menyimpan daftar org di localStorage.
- **Alert keamanan**: `lensio_cross_tenant_denied_total` + rule `CrossTenantAccessAttempts` dan `AuthenticationFailureSurge`, dengan unit test promtool termasuk kasus negatif.
- **Penegakan retensi**: sweep batch tiap jam untuk `ocr_requests` (90 hari), `usage_records` dan `audit_logs` (12 bulan) — sebelumnya PRIVACY.md menjanjikannya tanpa kode.
- **Validasi IaC di CI** (`tofu fmt` + `validate` semua modul & environment), yang langsung menemukan `for_each` invalid pada blok Gemini: nilainya berasal dari variabel `sensitive`, dan `for_each` menolak nilai bertanda.

### Security
- **Infra: `/metrics` tidak lagi publik**: dilindungi bearer `METRICS_TOKEN` (dibuat OpenTofu); production/staging menolak start tanpanya. Action `trivy-action` dipin ke SHA v0.36.0 dan `gosec` ke v2.29.0; input workflow rollback masuk lewat `env`, bukan interpolasi `${{ }}` di `run:` (script injection). Login Azure pakai OIDC, bukan JSON service principal.
- **Isolasi tenant (P0)**: `resolveOrgID` kini default-deny — `?org_id`/`org_id` body hanya diterima bila sama dengan org pemanggil (atau scope platform `*`, atau saat cek ReBAC SpiceDB langsung menyusul). Sebelumnya 9 endpoint (`/usage*`, `/account*`, `GET /auth/api-keys`) memakai nilai itu apa adanya, sehingga tenant lain bisa dibaca dan `PUT /account/plan` milik tenant lain bisa diubah. Signature helper diubah agar compiler memaksa semua call site ikut — guard sebelumnya hanya terpasang di 2 dari 9.
- **Bypass verifikasi email (P0)**: token kosong tidak lagi memverifikasi akun. Handler menolak token kosong dan cabang "dev flow" di `store.VerifyUserEmail` (yang mencocokkan hanya berdasarkan email) dihapus.
- **Eskalasi privilege (P0)**: role `admin` — diberikan ke setiap owner organisasi saat login — tidak lagi menjadi wildcard scope global di `HasScope`. Hanya `*` yang wildcard.
- **Brute force login (P0)**: rate limiter kini benar-benar terpasang pada `/api/v1/auth/{login,register,verify-email}`. Perbaikan sebelumnya mengubah `middleware/ratelimit.go` tetapi middleware-nya tidak pernah ada di rantai route publik. Timing login disamakan (bcrypt dummy) untuk menutup enumerasi akun.
- **PII at rest (P1)**: body respons yang di-cache idempotency dienkripsi AES-256-GCM (kunci turunan `SESSION_SECRET`) dan TTL turun 24 jam → 1 jam. Respons OCR memuat NIK/nama/alamat.
- **Revokasi sesi (P1)**: logout membatalkan token server-side via `users.sessions_valid_from` (migrasi 000014), bukan sekadar menghapus cookie.
- Hardening: `X-Forwarded-For` diambil dari hop paling kanan (tidak bisa dipalsukan pemanggil), `ENABLE_DEV_AUTH` wajib eksplisit untuk validator token tanpa tanda tangan, error internal tidak lagi dikembalikan ke klien, body error upstream Gemini tidak lagi masuk log, `/api/v1/account` non-GET ikut di-throttle.
### Changed
- **Dashboard membaca origin API saat runtime** (`/config.js`, ditulis container dari env `API_URL` saat start), menggantikan trik hostname `ca-dash-` → `ca-api-` yang rusak di custom domain. Satu image dashboard kini bisa dipromosikan dari staging ke production. Nilai `API_URL` divalidasi ketat (hanya origin); test menemukan bahwa `grep` per-baris meloloskan nilai bernewline, sehingga ditambah cek karakter. Proxy nginx sengaja tidak dipakai: semua pengguna akan berbagi satu bucket rate-limit login per-IP.
- CORS, `APP_BASE_URL`, dan `API_URL` dashboard diturunkan otomatis dari default domain environment Container Apps; custom domain cukup diisi di tfvars.

### Fixed
- **Infra tidak pernah bisa di-apply/di-deploy end-to-end** — diperbaiki:
  - Terragrunt tidak bisa diparse (tipe kondisional tidak konsisten) dan bentrok `required_providers` ganda → dihapus; kini satu stack `infra/live` + `staging.tfvars`/`production.tfvars`, state per environment.
  - Password DB acak memuat `# ? % :` yang memotong `DATABASE_URL` → kini alfanumerik 32 karakter (dibuktikan test `apply`).
  - Key Vault dekoratif (tidak ada yang membacanya) dan pasti 403 saat apply (firewall Deny + RBAC tanpa role deployer) → dihapus beserta identity dan subnet private endpoint.
  - CNAME Cloudflare menunjuk FQDN *revisi* (mati tiap deploy) dan tidak ada binding custom domain → kini FQDN ingress, sertifikat Origin CA, TXT `asuid`, dan `azurerm_container_app_custom_domain`.
  - Setting zona & ruleset WAF ditulis kedua stack pada zona yang sama → hanya production (`manage_cloudflare_zone`); rate limit kini menyertakan `cf.colo.id`.
  - Path image tofu (`ghcr.io/amirfaisalz/lensio-api`) ≠ yang di-push CI (`ghcr.io/amirfaisalz/lensio/lensio-api`); blok `registry` GHCR opsional ditambahkan.
  - Rollback tidak mungkin jalan: mode revisi `Single`, perintah `az containerapp revision set-traffic` tidak ada, `--target-revision previous` literal, `--app all` satu nama revisi → mode `Multiple`, `ingress traffic set`, `previous` di-resolve per app, `scripts/deploy.sh` menjaga revisi sebelumnya tetap hangat.
  - `tofu apply` me-revert image hasil deploy → `ignore_changes` untuk image/traffic; CD tidak lagi mengubah jumlah replika.
  - Produksi kini wajib lewat staging (+smoke test) dengan tag immutable yang sama; workflow keamanan kini memblokir release (dipanggil dari CI via `workflow_call`); `tofu test` berjalan di CI (3 dari 7 suite sebelumnya gagal tanpa ada yang tahu); parameter Postgres `connection_throttle.enable`; URL staging `staging-api.lensio.dev`.
- Kuota: hitung semua `POST /api/v1/ocr/*` (sebelumnya hanya KTP) di `GetMonthlyOCRCount`/`GetUsageSummary`.
- Idempotency: respons 429 tidak di-cache; lock di-release agar retry pasca-reset tidak replay 429 basi.
- Auth: tolak start prod/staging tanpa `SESSION_SECRET`/`DATABASE_URL`; cookie `Secure` paksa di production; guard cross-org `org_id` tanpa SpiceDB; scope sesi cookie di-allowlist.
- Rate limit: hapus override `?org_id` OIDC; login/register/verify-email masuk bucket IP (anti brute-force).
- Hardening: cap hash idempotency 6MB, fallback UUID/request-ID unik, validasi kontainer WebP, Enforcer fail-closed, error registrasi/verifikasi email dipropagasikan ke UI.
### Changed
- OCR handler: 6 handler (`ocr.go` 1941→±640 baris) dilebur ke satu pipeline generik + config per dokumen; perilaku byte-identik.
### Added
- Suite batas otorisasi: `apps/api/internal/http/handlers/tenant_isolation_test.go` (8 endpoint org-scoped menolak org asing), `apps/api/internal/http/auth_throttle_test.go`, `apps/api/internal/idempotency/seal_test.go`, tes revokasi sesi & panic recovery.
- `middleware.Recover`: panic handler jadi envelope 500 terkorelasi, bukan koneksi terputus tanpa respons.
- IaC: `SESSION_SECRET`, `OCR_PROVIDER`, `CORS_ALLOWED_ORIGINS`, `RATE_LIMIT_REPLICAS` kini diset dari OpenTofu (sebelumnya tidak ada — produksi akan diam-diam memakai mock engine dan tanpa header CORS). `RATE_LIMIT_ENABLED` yang tidak pernah dibaca kode dihapus.
- OpenAPI: parameter `org_id` + respons `403` (`insufficient_scope` / `permission_denied`) didokumentasikan; `permission_denied` ditambahkan ke enum `ErrorEnvelope`.
- `docs/audits/2026-09-14-implementation-audit.md`: temuan + status perbaikan audit security/wiring.
### Changed
- Gate kualitas: `gofmt` kini ditegakkan di CI dan `.githooks/pre-commit` (sebelumnya 20 file drift tanpa terdeteksi); `golangci-lint@latest` dipin ke `v1.64.8`; base image builder dipin ke `golang:1.26-alpine`.
- Default model Gemini hanya didefinisikan di `services/ocr/providers/gemini.go`; `config` tidak lagi menduplikasinya.

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
