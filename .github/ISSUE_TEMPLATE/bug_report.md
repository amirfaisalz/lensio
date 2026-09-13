---
name: Bug report
about: Laporkan bug yang reproduksibel
title: "fix: "
labels: bug
---

## Deskripsi
<!-- Apa yang terjadi vs yang diharapkan -->

## Reproduksi
<!-- Langkah minimal, gunakan HANYA data sintetis (tests/fixtures/synthetic). Jangan lampirkan KTP asli/kunci asli. -->
1.
2.

## Lingkungan
- Commit:
- Go / Node:
- Cara jalan (`docker compose` / `go run`):

## Log (bersihkan PII dulu)
```
```

## Checklist
- [ ] Hooks aktif (`git config core.hooksPath .githooks`)
- [ ] `go test -race -cover ./...` sudah dijalankan
