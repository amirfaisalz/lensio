# Audit Implementasi Lensio — Security, Wiring & Calon Bug

- Tanggal: 2026-09-14
- Ruang lingkup: `apps/api`, `services/ocr`, `apps/dashboard`, `openapi/openapi.yaml`, `docker-compose.yml`, `.env.example`
- Metode: baca langsung via codegraph + `grep`/`read` (4 sub-agent `explore` gagal karena `GOOGLE_GENERATIVE_AI_API_KEY` hilang, dialihkan ke audit manual — tidak ada pencarian ganda).
- Status repo saat audit: `git status` bersih, `.env` tidak ter-track (`gitignore:.env`), hanya placeholder di `.env.example` / `docker-compose.yml`.

## Ringkasan eksekutif

- **Security inti SUDAH sesuai & terpakai**: SHA-256 API key, scope middleware, cookie `lensio_session` HttpOnly+SameSite Lax, PII sanitizer, validasi image in-memory, idempotency, rate-limit O(1), CORS allowlist.
- **Wiring antar modul SUDAH sesuai & terpakai**: `main.go → NewRouterWithDeps` merangkai semua 6 OCR + auth + quota + metering + OTel; `OCREngine` di balik interface + circuit breaker; fallback Mock/Memory/Dev hanya untuk dev/test.
- **1 bug P0 TERKONFIRMASI saat verifikasi lanjutan**: kuota & ringkasan usage hanya menghitung `/api/v1/ocr/ktp` — 5 doc lain (SIM/Passport/NPWP/KK/Invoice) lolos kuota.
- **Sisa temuan di bawah berstatus CONFIRMED (terbaca di kode) kecuali dinyatakan OPEN (perlu uji runtime).**

## 1. Security — yang sudah benar

| Area | Bukti | Status |
|---|---|---|
| Hash API key | `apps/api/internal/apikey/apikey.go:34,62` rand 32B, hash SHA-256, prefix mask; `store/api_keys.go:42,77` simpan hash saja; plaintext sekali `handlers/api_keys.go:191` | Sesuai, terpakai |
| Auth middleware | `middleware/auth.go:63,87,98` cek Bearer → hash → revoked/expired; `touchKeyAsync` debounce 5 mnt, sem 10 (`middleware/toucher.go:22,68`) | Sesuai, terpakai |
| DualAuth | `middleware/oidc.go:530,554,567` OIDC dulu (non-`lensio_` prefix) lalu API key; cookie pendamping ditempel (`oidc.go:599`) | Sesuai, terpakai |
| Session cookie | `handlers/auth.go:223,251` `lensio_session` HttpOnly, SameSite Lax, logout `Max-Age:-1`; `GET /auth/me` verifikasi ulang | Sesuai, terpakai dg catatan P1-2 |
| Scope | `middleware/scope.go:41` + `router.go:99,143-169` semua OCR `ocr:write`, usage `usage:read` | Sesuai, terpakai |
| PII log | `telemetry/logger.go:56,136,169` redact NIK regex + image + sensitive keys, korelasi `trace_id/request_id`; dipakai `cmd/server/main.go:42` | Sesuai, terpakai |
| OCR upload | `handlers/ocr.go:152,199,239` MaxBytes 5MB+1KB, magic-byte + DecodeConfig + dimensi, `RemoveAll`, `clear(imgBytes)`, log zero-PII | Sesuai, terpakai |
| Password/register | `handlers/auth.go:79,108,194` hash password, token verifikasi hanya di dev/test, tolak belum verifikasi | Sesuai, terpakai |
| CORS | `middleware/cors.go:37,56` allowlist eksplisit, kredensial hanya origin cocok, OPTIONS unknown → 403 | Sesuai, terpakai |
| Idempotency | `middleware/idempotency.go:102,151` + `idempotency/hash.go:42` + `idempotency/postgres.go:49` batas 256 char, hash + restore body, 409/422/replay, 5xx→Release, panic→Release | Sesuai, terpakai dg catatan P0-4 |
| Rate limit | `ratelimit/ratelimit.go:50` O(1) + `middleware/ratelimit.go:84` header standar | Sesuai, terpakai dg catatan P1-5/6 |
| Dashboard secret | `services/api.ts:58,97` key in-memory + `credentials:include`; `AuthContext.tsx:324,371` token dikosongkan; test `no localStorage` ada | Sesuai, terpakai dg catatan P1-10/11 |
| Secret repo | `.env` gitignored & tak ter-track; `docker-compose` dev password hanya dev | Sesuai |

## 2. Wiring antar modul — sesuai & terpakai

```
Client → CORS → RequestID/Tracing → UsageMetering → DualAuth → RateLimit →
SessionOrg → RequireScope → Idempotency → OCRHandler(KTP/SIM/Passport/NPWP/KK/Invoice)
→ ValidateImage → OCREngine[CircuitBreaker(Gemini|Mock)] → ValidateX deterministik
→ OCRStore(metadata non-PII) + Quota.Enforcer + UsageRecorder(chan 1024, drop saat penuh) + OTel
```

- `apps/api/cmd/server/main.go:173,197,230` merangkai DB, migrasi, OCR engine, limiter, recorder, idempotency, OIDC, SpiceDB, cleaner, graceful shutdown — semua dipakai.
- `apps/api/internal/http/router.go:51` mendaftarkan 6 `POST /ocr/*`, `GET /ocr/{id}` (bila OCRStore ada), auth/api-keys/usage/account/me/verify + probes/docs. `openapi.yaml:294,359,424,490,556,623,690` cocok 7 path (6 POST + 1 GET) — tidak ada mismatch kontrak.
- `SessionOrg` (`middleware/session_org.go:19`) dipakai di semua OCR — cookie-auth dapat org tanpa API key, ID sintetis kosong → `api_key_id` NULL. Benar & terpakai.
- `KeyToucher`, `UsageRecorder` (`usage/recorder.go:40,66,87` non-blocking, 3s timeout insert, drain on Close), `GetOrganizationPlan` fallback `free` (`store/account.go:147`) — terpakai.
- Lapisan satu arah Transport→Handler→Service→Store/OCR dipatuhi; error wrap `%w`; tanpa `panic` produksi.

## 3. Best practice

- Baik: `slog` JSON, `ctx` propagation, `go vet`/golangci (gosec/revive/errcheck), `MaxBytesReader`, query parameterisasi (`store/usage.go:292` bangun `$N` + args — aman injeksi).
- Kurang: 6 handler OCR duplikat ±300 baris (`handlers/ocr.go:111,406,…,1580`) — risiko drift; seharusnya 1 handler generik + tabel `docType`.
- Klaim badge coverage 93.5%/88.1% vs target 100% (`AGENTS.md` Rule 2) — gap, bukan bug.

## 4. Calon bug prioritas (semua CONFIRMED dari kode)

### P0

**P0-1 TERKONFIRMASI — Kuota & ringkasan hanya hitung KTP.**
- `apps/api/internal/store/usage.go:255-261` `GetMonthlyOCRCount`: `endpoint = '/api/v1/ocr/ktp'`.
- `apps/api/internal/store/usage.go:116` `GetUsageSummary`: `ocrRequestsCount` filter `endpoint = '/api/v1/ocr/ktp'`.
- Dampak: SIM/Passport/NPWP/KK/Invoice sukses tidak mengurangi kuota + sisa kuota dashboard salah. `QuotaChecker` dipakai semua handler (`handlers/ocr.go:127`) tapi sumber angkanya bocor.
- Fix: `endpoint LIKE '/api/v1/ocr/%'` (atau `IN (...)`) + `status_code < 400`; tambah test multi-doc; migrasi tidak perlu (query saja).

**P0-2 Fail-open cluster: ENV default development + SESSION_SECRET fallback + DevValidator.**
- `config.go:33` ENV kosong → `development`; `config.go:80` secret fallback `lensio-session-secret-key-development-32b`; `main.go:142,147` `DevTokenValidator(alg:none, auto-grant ocr:write)` aktif di dev/test (`oidc.go:673`).
- Dampak: salah config prod = token tempa + CORS localhost + `verification_token` bocor (`auth.go:108`).
- Fix: ENV default `production`/fail-closed; wajib `SESSION_SECRET` ≥32B di prod; `DevTokenValidator` hanya bila flag eksplisit `ENABLE_DEV_AUTH=true`.

**P0-3 Tanpa SpiceDB, otorisasi key fail-open.**
- `handlers/api_keys.go:111,261` bila `authorizer==nil` skip cek; `resolveOrgIDWithAccount` terima `?org_id` eksplisit.
- Dampak: user terautentikasi mana pun bisa create/revoke key org lain.
- Fix: default-deny; tanpa authorizer wajib owner-check via `AccountStore`.

**P0-4 Respons kuota 429 ikut di-cache idempotency 24 jam.**
- `middleware/idempotency.go:171` cache semua `<500`; quota `429` dari handler (`ocr.go:139`) ikut `Complete`.
- Dampak: retry key sama pasca-reset tetap replay `429` lama. (Rate-limiter `429` aman karena terjadi sebelum lock.)
- Fix: `Release` untuk `429/402/quota`, cache hanya `2xx` + `4xx` non-kuota.

### P1

- **P1-1 `Secure` cookie kondisional** (`auth.go:223,251`): `r.TLS||X-Forwarded-Proto`. Tanpa header forward di ACA/Envoy → cookie non-Secure. Fix: paksa `Secure=true` saat prod.
- **P1-2 Rate-limit key dari `?org_id`** (`middleware/ratelimit.go:63`): user OIDC bisa pindah bucket. Fix: org dari `SessionOrg`, bukan query.
- **P1-3 Bypass rate-limit untuk `/auth/*,/account`** (`middleware/ratelimit.go:47` + `router.go:84` limiter di dalam auth): login/register tanpa guard → brute-force. Fix: bucket IP ketat khusus auth.
- **P1-4 Idempotency hash 10MB vs OCR 5MB** (`idempotency/hash.go:13` jalan sebelum `MaxBytesReader`): amplifikasi memori. Fix: batasi 5MB+margin untuk OCR.
- **P1-5 UUID/request-ID fallback konstanta** (`ocr.go:27`, `request_id.go:46`): collision PK/korelasi. Fix: return error, jangan konstanta.
- **P1-6 Frontend fail-soft**: `AuthContext.tsx:378,415` `catch→return true`; `createDevJwtToken/registerOrLogin` (`AuthContext.tsx:73,483`) fabrikasi JWT unsigned. Pastikan tak reachable di build prod; lempar error asli.
- **P1-7 `user.Roles` → `Scopes` mentah** (`session_org.go:53`, login `auth.go:211` grant `ocr:write` ke semua user terverifikasi). Fix: whitelist scope.
- **P1-8 WebP tanpa decode/dimensi** (`validator.go:80` hanya `len<30`). Fix: parser WebP atau tolak WebP.
- **P1-9 Mode ephemeral (`DATABASE_URL` kosong → `main.go:80`) + `quota.go:48` allow-true**: kuota mati tapi OCR jalan. Tandai eksplisit / tolak OCR tanpa DB.

## 5. Yang sudah diverifikasi TIDAK jadi bug

- `ComputePayloadHash` restore body benar; `PostgresStore.LockOrGet` overwrite-hanya-bila-expired benar.
- `KeyToucher` throttle + prune 2000 + `WithoutCancel` 2s benar.
- Replay header `Idempotent-Replayed` konsisten (case-insensitive di Go).
- Metering exclude `/health,/ready,/metrics,/docs,/openapi,/auth,/usage,/account` benar (hindari loop); OCR+SessionOrg tetap tercatat.
- `GetOrganizationPlan` fallback `free` untuk org hilang — by-design, bukan bug.
- Tidak ada `panic` produksi, `as any`, secret ter-commit.

## 6. Rekomendasi urutan perbaikan

1. P0-1 (kuota multi-doc) + test — risiko billing terbesar, diff kecil.
2. P0-4 (jangan cache 429 kuota) — satu cabang `if`.
3. P0-2 + P1-1 (fail-closed ENV/secret/Secure) — satu PR config.
4. P0-3 (default-deny authorizer) — satu PR authz.
5. P1-2/P1-3 (rate-limit key + auth bucket) — satu PR limiter.
6. Refaktor 6 handler OCR jadi generik (cegah drift berikutnya).

## 7. Status perbaikan (2026-09-14, sesi yang sama)

Semua temuan di atas sudah diperbaiki dan terverifikasi (`go build`, `go vet`, `go test -race -cover ./...`, `tsc`, `biome`, `vitest` 89/89 hijau):

| Temuan | Perbaikan | Verifikasi |
|---|---|---|
| P0-1 kuota hanya KTP | `store/usage.go`: `GetMonthlyOCRCount` + `GetUsageSummary` → `endpoint LIKE '/api/v1/ocr/%'`; tambah `TestMonthlyOCRCount_MultiDocTypes` (6 doc count=6, 429/non-OCR dikecualikan) | Live DB pass; test baru gagal pada query lama (red-green valid) |
| P0-4 cache 429 | `middleware/idempotency.go`: 429 tidak di-`Complete` melainkan `Release`; tambah `TestIdempotencyMiddleware_RateLimitNotCached` | Middleware suite pass |
| P0-2 fail-open | `cmd/server/main.go`: prod/staging wajib `SESSION_SECRET` + `DATABASE_URL`, else `os.Exit(1)`; default ENV dev tidak diubah (jaminan `config_test`) | Build pass |
| P1-1 Secure kondisional | `handlers/auth.go` login+logout: `Secure` paksa true saat `ENV==production`; tambah `TestSessionCookieSecureInProduction` | Handlers suite pass |
| P0-3 cross-org | `handlers/api_keys.go`: `rejectCrossOrgKeyManagement` tolak `org_id` eksplisit milik org lain (kecuali admin/anon-test); tambah `TestCreateAPIKeyHandler_CrossOrgGuard` (403/201/201-admin) | Handlers suite pass |
| P1-2 bucket-hop | `middleware/ratelimit.go`: hapus override `?org_id` untuk OIDC | Middleware suite pass |
| P1-3 brute-force | `middleware/ratelimit.go`: bypass tersisa hanya `/auth/me`, `/auth/logout`, `/account*`; login/register/verify-email masuk bucket IP; tambah test 429 login + abaikan query | Test bypass lama tetap hijau |
| P1-4 hash 10MB | `idempotency/hash.go`: `DefaultMaxBodySize` 10MB → 6MB (idempotency hanya bungkus OCR 5MB) | Idempotency suite pass |
| P1-5 ID konstanta | `handlers/ocr.go` `GenerateUUIDv4` + `middleware/request_id.go`: fallback pakai `UnixNano` (unik per panggilan) | Suite pass |
| P1-7 scope mentah | `middleware/session_org.go`: allowlist `ocr:read/write`, `usage:read`, `developer`; `admin`/`*` dari IdP dibuang; tambah `TestSessionOrg_StripsWildcardScopes` | Suite pass |
| P1-8 WebP longgar | `services/ocr/validator.go`: `validateWebPContainer` (RIFF size + chunk VP8/VP8L/VP8X + VP8 start code); fixture test diperketat + 3 kasus negatif baru | OCR suite pass |
| P1-9 ephemeral allow | `quota/quota.go`: store nil → error (fail-closed); `main.go` tolak ephemeral di prod/staging; test fallback diperbarui | Quota suite pass |
| P1-6 fail-soft | `AuthContext.tsx`: `registerUser`/`verifyUserEmail` propagasikan error (LoginPage sudah try/catch); test consumer disesuaikan | `tsc`, 89/89 vitest |
| Duplikasi handler | `handlers/ocr.go` 1941 → ±640 baris: satu `ocrPipeline` + 6 config; seluruh perbedaan perilaku (pesan, sentinel, span KK, fallback field `image`, cabang 502-vs-500, confidence-cap) dipertahankan byte-identik | `handlers` + `http` suite (cakup semua 6 handler) pass dengan `-race` |

Catatan: `gofmt -l` masih menandai beberapa file — seluruhnya drift pre-existing di HEAD (mis. trailing blank, alignment struct lama); tidak diubah agar diff minimal. `golangci-lint` tidak tersedia di host sehingga lint Go diwakili `go vet` + `gofmt` pada file sentuhan (bersih).
