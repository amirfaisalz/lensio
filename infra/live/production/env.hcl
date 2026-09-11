locals {
  environment                    = "production"
  location                       = "southeastasia"
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
  api_subdomain                  = "api"
  dashboard_subdomain            = "dashboard"
}
