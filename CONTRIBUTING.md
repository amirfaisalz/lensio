# Contributing to Lensio

Thanks for considering a contribution. Read `AGENTS.md` first — its five mandatory rules apply to every change.

## Workflow
1. Fork, branch dari `main` (`feat/...`, `fix/...`, `docs/...`).
2. Aktifkan hooks: `git config core.hooksPath .githooks`.
3. Ikuti pola kode yang ada (`Go net/http`, `log/slog`, error wrap `%w`, tanpa `panic` di jalur produksi).
4. Gunakan **data sintetis saja** — jangan pernah commit KTP asli, kunci asli, atau dump DB. Lihat `USE_POLICY.md`.

## Pre-commit (4 lapis, otomatis saat commit)
1. Build/typecheck (`go build ./...`, `tsc --noEmit`)
2. Lint (`golangci-lint`, `go vet`, frontend lint)
3. Test race+coverage (`go test -race -cover ./...`)
4. Guardrail Big-O, keamanan & PII

Jangan `--no-verify`. Jangan `git push` otomatis ke upstream tanpa diminta — buka PR saja.

## Pull request
- Deskripsikan masalah, pendekatan minimal (YAGNI), dan bukti verifikasi (diagnostik/lint/test).
- Perbarui docs yang terdampak (`README`, ADR baru bila keputusan arsitektur, `CHANGELOG.md` → `Unreleased`).
- Satu PR = satu perubahan logis. CI harus hijau (termasuk 5 security gates).

## Melaporkan bug / keamanan
- Bug biasa: gunakan issue template.
- Dugaan kebocoran/vulnerability: **jangan** buka issue publik — ikut `SECURITY.md`.
