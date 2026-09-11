terraform {
  required_version = ">= 1.8.0"
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
  }
}

provider "azurerm" {
  features {
    key_vault {
      purge_soft_delete_on_destroy    = false
      recover_soft_deleted_key_vaults = true
    }
  }
}

provider "cloudflare" {}

locals {
  common_tags = merge(var.tags, {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "OpenTofu"
  })
}

module "networking" {
  source      = "../../modules/networking"
  project     = var.project
  environment = var.environment
  location    = var.location
  tags        = local.common_tags
}

module "key_vault" {
  source                      = "../../modules/key_vault"
  project                     = var.project
  environment                 = var.environment
  location                    = module.networking.location
  resource_group_name         = module.networking.resource_group_name
  tenant_id                   = var.tenant_id
  vnet_id                     = module.networking.vnet_id
  private_endpoints_subnet_id = module.networking.private_endpoints_subnet_id
  tags                        = local.common_tags
}

module "postgres" {
  source                 = "../../modules/postgres"
  project                = var.project
  environment            = var.environment
  location               = module.networking.location
  resource_group_name    = module.networking.resource_group_name
  vnet_id                = module.networking.vnet_id
  delegated_subnet_id    = module.networking.postgres_subnet_id
  admin_password         = module.key_vault.db_admin_password
  sku_name               = var.postgres_sku
  storage_mb             = var.postgres_storage_mb
  high_availability_mode = var.postgres_ha_mode
  backup_retention_days  = var.postgres_backup_retention_days
  tags                   = local.common_tags
}

module "container_apps" {
  source                   = "../../modules/container_apps"
  project                  = var.project
  environment              = var.environment
  location                 = module.networking.location
  resource_group_name      = module.networking.resource_group_name
  infrastructure_subnet_id = module.networking.container_apps_subnet_id
  database_url             = module.postgres.connection_string
  api_image                = var.api_image
  dashboard_image          = var.dashboard_image
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
  count                 = var.cloudflare_zone_id != "" ? 1 : 0
  source                = "../../modules/cloudflare"
  zone_id               = var.cloudflare_zone_id
  environment           = var.environment
  api_subdomain         = var.api_subdomain
  dashboard_subdomain   = var.dashboard_subdomain
  api_target_fqdn       = module.container_apps.api_fqdn
  dashboard_target_fqdn = module.container_apps.dashboard_fqdn
}
