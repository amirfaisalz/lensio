environment                    = "production"
postgres_sku                   = "GP_Standard_D2ds_v5"
postgres_storage_mb            = 65536
postgres_ha_mode               = "ZoneRedundant"
postgres_backup_retention_days = 35
postgres_geo_redundant_backups = true
api_min_replicas               = 2
api_max_replicas               = 10
api_cpu                        = 1.0
api_memory                     = "2.0Gi"
dashboard_min_replicas         = 2
dashboard_max_replicas         = 5
dashboard_cpu                  = 0.5
dashboard_memory               = "1.0Gi"

# Resend (smtp.resend.com:587, STARTTLS; net/smtp cannot do implicit TLS on 465).
# The domain must be verified in Resend; host/username/password come from
# GitHub (vars.SMTP_HOST, vars.SMTP_USERNAME, secrets.SMTP_PASSWORD).
smtp_from = "noreply@lensio.tec.my.id"

# No custom domain yet: the apps are served on their *.azurecontainerapps.io
# FQDNs, and CORS, APP_BASE_URL and the dashboard's API_URL are derived from
# them automatically. Once lensio.tec.my.id is bound (infra/live/README.md), set:
#   api_public_url       = "https://api.lensio.tec.my.id"
#   cors_allowed_origins = ["https://lensio.tec.my.id"]
#   app_base_url         = "https://lensio.tec.my.id"
