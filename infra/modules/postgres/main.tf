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
  }
}

locals {
  name_prefix = "${var.project}-${var.environment}"
  common_tags = merge(var.tags, {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "OpenTofu"
  })
}

resource "random_string" "server_suffix" {
  length  = 6
  special = false
  upper   = false
}

# Alphanumeric only: the password is interpolated into DATABASE_URL, where
# characters like # ? % : would cut the URL short. 32 chars of [A-Za-z0-9] is
# ~190 bits and still meets Azure's three-of-four character class rule.
resource "random_password" "admin" {
  length      = 32
  special     = false
  min_upper   = 1
  min_lower   = 1
  min_numeric = 1
}

resource "azurerm_private_dns_zone" "postgres" {
  name                = "${local.name_prefix}.postgres.database.azure.com"
  resource_group_name = var.resource_group_name
  tags                = local.common_tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "postgres" {
  name                  = "vnet-link-psql-${local.name_prefix}"
  resource_group_name   = var.resource_group_name
  private_dns_zone_name = azurerm_private_dns_zone.postgres.name
  virtual_network_id    = var.vnet_id
  tags                  = local.common_tags
}

resource "azurerm_postgresql_flexible_server" "postgres" {
  name                   = "psql-${local.name_prefix}-${random_string.server_suffix.result}"
  resource_group_name    = var.resource_group_name
  location               = var.location
  version                = var.postgres_version
  delegated_subnet_id    = var.delegated_subnet_id
  private_dns_zone_id    = azurerm_private_dns_zone.postgres.id
  administrator_login    = var.admin_username
  administrator_password = random_password.admin.result
  sku_name               = var.sku_name
  storage_mb             = var.storage_mb
  backup_retention_days  = var.backup_retention_days

  geo_redundant_backup_enabled = var.geo_redundant_backups_enabled

  dynamic "high_availability" {
    for_each = var.high_availability_mode != "Disabled" ? [1] : []
    content {
      mode = var.high_availability_mode
    }
  }

  tags = local.common_tags

  # Azure picks the zones and swaps them on HA failover; without this every plan
  # after a failover would try to move the server back.
  lifecycle {
    ignore_changes = [zone, high_availability[0].standby_availability_zone]
  }

  depends_on = [
    azurerm_private_dns_zone_virtual_network_link.postgres
  ]
}

resource "azurerm_postgresql_flexible_server_database" "app_db" {
  name      = var.database_name
  server_id = azurerm_postgresql_flexible_server.postgres.id
  collation = "en_US.utf8"
  charset   = "UTF8"
}

resource "azurerm_postgresql_flexible_server_configuration" "require_secure_transport" {
  name      = "require_secure_transport"
  server_id = azurerm_postgresql_flexible_server.postgres.id
  value     = "ON"
}

resource "azurerm_postgresql_flexible_server_configuration" "connection_throttling" {
  # Flexible Server name; "connection_throttling" is the retired Single Server one.
  name      = "connection_throttle.enable"
  server_id = azurerm_postgresql_flexible_server.postgres.id
  value     = "ON"
}

resource "azurerm_postgresql_flexible_server_configuration" "log_connections" {
  name      = "log_connections"
  server_id = azurerm_postgresql_flexible_server.postgres.id
  value     = "ON"
}
