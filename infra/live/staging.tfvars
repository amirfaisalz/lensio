environment                    = "staging"
postgres_sku                   = "B_Standard_B1ms"
postgres_storage_mb            = 32768
postgres_ha_mode               = "Disabled"
postgres_backup_retention_days = 7
api_min_replicas               = 1
api_max_replicas               = 2
api_cpu                        = 0.5
api_memory                     = "1.0Gi"
dashboard_min_replicas         = 1
dashboard_max_replicas         = 2
dashboard_cpu                  = 0.25
dashboard_memory               = "0.5Gi"
api_subdomain                  = "staging-api"
dashboard_subdomain            = "staging-dashboard"

# No custom domain yet: the apps are served on their *.azurecontainerapps.io
# FQDNs, and CORS, APP_BASE_URL and the dashboard's API_URL are derived from
# them automatically. Once staging.lensio.tec.my.id is bound (infra/live/README.md), set:
#   api_public_url       = "https://api.staging.lensio.tec.my.id"
#   cors_allowed_origins = ["https://staging.lensio.tec.my.id"]
#   app_base_url         = "https://staging.lensio.tec.my.id"
