# Lensio DORA Elite Targets — Evidence Map

> Target hiring bar: deploys on demand, lead time < 1 day, change failure rate ≤ 5%, recovery < 1 hour.
> This file maps each DORA metric to where Lensio proves it. No new process — just pointers.

| DORA Metric | Elite Target | Lensio Evidence | Reference |
|---|---|---|---|
| **Deployment frequency** | On demand | GitHub Actions `ci.yml` → `deploy.yml`: every `main` push runs 6 gates, staging auto-deploys, production promotes on smoke-pass. No manual clicks. | `.github/workflows/ci.yml`, `.github/workflows/deploy.yml` |
| **Lead time for changes** | < 1 day | Small PR → 6 automated gates (vet, `go test -race`, govulncheck, gosec, Spectral, Vitest) → staging smoke (`scripts/smoke-test.sh`) → production. Evidence run captured with commit SHA + timestamps. | `docs/deployment/evidence-log.md` §3 |
| **Change failure rate** | ≤ 5% | Broken revision drill: faulty `/health` fails smoke (exit 1), `deploy-production` gate (`needs: deploy-staging`, `if: success()`) never fires. Bad change cannot reach prod. | `docs/deployment/evidence-log.md` §4, `docs/incidents/INC-20260912-03-broken-deployment-smoke-test.md` |
| **Failed deployment recovery** | < 1 hour | Emergency rollback via immutable ACA revision traffic shift: **1.017s** measured (target < 60s, 98.3% margin). Runbook + script + post-rollback `/health`+`/ready` verification. | `docs/deployment/evidence-log.md` §5, `docs/rollback.md`, `scripts/rollback.sh` |

## SLO backing

Availability SLO 99.9% (43.2 min/month budget), latency P95 < 500ms gateway / < 2s OCR, multi-burn-rate alerts. Recovery drills feed the error budget, not anecdotes.

See `docs/observability/slo-definition.md` and `infra/observability/prometheus/alerting_rules.yml`.
