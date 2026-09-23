terraform {
  required_version = ">= 1.9.0"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.116"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.30"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  # One stack, one state per environment: pass the key at init, e.g.
  #   tofu init -backend-config=key=staging.tfstate
  backend "azurerm" {
    resource_group_name  = "rg-lensio-tfstate"
    storage_account_name = "stlensiotfstate"
    container_name       = "tfstate"
    # The state account has shared-key access disabled: Entra ID only.
    use_azuread_auth = true
  }
}

provider "azurerm" {
  features {}
}

# Reads CLOUDFLARE_API_TOKEN (Zone DNS/Settings/WAF edit + SSL and Certificates edit).
provider "cloudflare" {}

locals {
  common_tags = merge(var.tags, {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "OpenTofu"
  })
}

module "networking" {
  source      = "../modules/networking"
  project     = var.project
  environment = var.environment
  location    = var.location
  tags        = local.common_tags
}

module "postgres" {
  source                        = "../modules/postgres"
  project                       = var.project
  environment                   = var.environment
  location                      = module.networking.location
  resource_group_name           = module.networking.resource_group_name
  vnet_id                       = module.networking.vnet_id
  delegated_subnet_id           = module.networking.postgres_subnet_id
  sku_name                      = var.postgres_sku
  storage_mb                    = var.postgres_storage_mb
  high_availability_mode        = var.postgres_ha_mode
  backup_retention_days         = var.postgres_backup_retention_days
  geo_redundant_backups_enabled = var.postgres_geo_redundant_backups
  tags                          = local.common_tags
}

module "container_apps" {
  source                   = "../modules/container_apps"
  project                  = var.project
  environment              = var.environment
  location                 = module.networking.location
  resource_group_name      = module.networking.resource_group_name
  infrastructure_subnet_id = module.networking.container_apps_subnet_id
  database_url             = module.postgres.connection_string
  gemini_api_key           = var.gemini_api_key
  ocr_provider             = var.ocr_provider
  cors_allowed_origins     = var.cors_allowed_origins
  smtp_host                = var.smtp_host
  smtp_username            = var.smtp_username
  smtp_password            = var.smtp_password
  smtp_from                = var.smtp_from
  app_base_url             = var.app_base_url
  api_public_url           = var.api_public_url
  registry_username        = var.registry_username
  registry_password        = var.registry_password
  api_cpu                  = var.api_cpu
  api_memory               = var.api_memory
  api_min_replicas         = var.api_min_replicas
  api_max_replicas         = var.api_max_replicas
  dashboard_cpu            = var.dashboard_cpu
  dashboard_memory         = var.dashboard_memory
  dashboard_min_replicas   = var.dashboard_min_replicas
  dashboard_max_replicas   = var.dashboard_max_replicas
  tags                     = local.common_tags
}

module "cloudflare" {
  count                        = var.cloudflare_zone_id != "" ? 1 : 0
  source                       = "../modules/cloudflare"
  zone_id                      = var.cloudflare_zone_id
  environment                  = var.environment
  manage_zone                  = var.manage_cloudflare_zone
  proxied                      = var.cloudflare_proxied
  api_subdomain                = var.api_subdomain
  dashboard_subdomain          = var.dashboard_subdomain
  api_target_fqdn              = module.container_apps.api_fqdn
  dashboard_target_fqdn        = module.container_apps.dashboard_fqdn
  container_app_environment_id = module.container_apps.environment_id
  api_app_id                   = module.container_apps.api_app_id
  dashboard_app_id             = module.container_apps.dashboard_app_id
  api_verification_id          = module.container_apps.api_custom_domain_verification_id
  dashboard_verification_id    = module.container_apps.dashboard_custom_domain_verification_id
}
