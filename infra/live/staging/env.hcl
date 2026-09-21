locals {
  environment                    = "staging"
  location                       = "southeastasia"
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

  # Serving fixture data to paying callers is worse than failing loudly, so this
  # is set explicitly rather than left to the "mock" default.
  ocr_provider = "gemini_flash"

  # The dashboard is a separate origin; without it the API emits no CORS headers.
  cors_allowed_origins = ["https://staging-dashboard.lensio.dev"]
}
