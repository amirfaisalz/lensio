include "root" {
  path = find_in_parent_folders()
}

locals {
  env_vars = read_terragrunt_config("env.hcl").locals
}

inputs = {
  environment                    = local.env_vars.environment
  location                       = local.env_vars.location
  postgres_sku                   = local.env_vars.postgres_sku
  postgres_storage_mb            = local.env_vars.postgres_storage_mb
  postgres_ha_mode               = local.env_vars.postgres_ha_mode
  postgres_backup_retention_days = local.env_vars.postgres_backup_retention_days
  postgres_geo_redundant_backups = local.env_vars.postgres_geo_redundant_backups
  api_min_replicas               = local.env_vars.api_min_replicas
  api_max_replicas               = local.env_vars.api_max_replicas
  api_cpu                        = local.env_vars.api_cpu
  api_memory                     = local.env_vars.api_memory
  dashboard_min_replicas         = local.env_vars.dashboard_min_replicas
  dashboard_max_replicas         = local.env_vars.dashboard_max_replicas
  dashboard_cpu                  = local.env_vars.dashboard_cpu
  dashboard_memory               = local.env_vars.dashboard_memory
  ocr_provider                   = local.env_vars.ocr_provider
  cors_allowed_origins           = local.env_vars.cors_allowed_origins
  api_subdomain                  = local.env_vars.api_subdomain
  dashboard_subdomain            = local.env_vars.dashboard_subdomain
}
