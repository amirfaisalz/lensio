# Security Policy

## Supported versions
`main` saja yang didukung. Versi lain tidak menerima patch.

## Reporting a vulnerability
Jangan buka issue publik untuk dugaan kebocoran. Kirim email ke **security@lensio.dev** dengan: deskripsi, langkah reproduksi minimal (data sintetis saja), dampak, dan commit yang terdampak. Target respons awal: 3x24 jam. Jangan lampirkan KTP asli, kunci asli, atau dump DB.

## Secrets
- Repo ini tidak berisi secret prod. Kredensial di `docker-compose.yml` / `.env.example` adalah **dev-only** (`lensio_dev_password`, `admin/admin`, `lensio_spicedb_key`).
- Prod wajib injeksi via environment / Azure Key Vault. Gitleaks berjalan di CI tanpa allowlist regex global: pola `lensio_live_*`, `AIzaSy*`, dan password dev di luar path fixture/test akan menggagalkan CI.

## Scope
API key SHA-256 + scope (`ocr:read/ocr:write/usage:read`), rate-limit per identitas (org/OIDC/IP), cookie `lensio_session` (`HttpOnly, SameSite=Lax, Secure` di HTTPS), ReBAC SpiceDB untuk mutasi kunci, audit log append-only.
