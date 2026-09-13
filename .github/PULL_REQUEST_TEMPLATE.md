<!-- Satu PR = satu perubahan logis. Baca CONTRIBUTING.md dulu. -->

## Ringkasan
<!-- Apa + kenapa (1-2 kalimat) -->

## Perubahan
-
-

## Verifikasi
- [ ] `go build ./...` / `tsc --noEmit`
- [ ] `golangci-lint run ./...` + `go vet ./...`
- [ ] `go test -race -cover ./...` hijau
- [ ] Coverage tidak turun / docs diperbarui (`CHANGELOG.md` bila perlu)

## Keamanan & PII
- [ ] Tanpa KTP asli, kunci asli, atau secret (cek `git diff`)
- [ ] Tanpa token/PII di `localStorage` (frontend)
- [ ] Log baru lolos sanitizer PII
