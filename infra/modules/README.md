# Infrastructure Modules

Reusable OpenTofu modules composed by `infra/live`. Each has its own `tests/*.tftest.hcl` (`tofu test` in the module directory, run by CI):

| Module | Owns |
|---|---|
| `networking` | Resource group, VNet, Container Apps and PostgreSQL subnets, NSGs |
| `postgres` | PostgreSQL 16 Flexible Server on a delegated subnet with private DNS; generates the admin password |
| `container_apps` | Log Analytics, Container Apps environment, API and dashboard apps; generates `SESSION_SECRET` and `METRICS_TOKEN` |
| `cloudflare` | DNS, Origin CA certificate, custom domain binding, and (for the zone owner) zone settings and WAF rulesets |
