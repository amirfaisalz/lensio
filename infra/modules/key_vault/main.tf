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

resource "random_string" "kv_suffix" {
  length  = 6
  special = false
  upper   = false
}

resource "random_password" "db_password" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "random_password" "jwt_secret" {
  length  = 48
  special = false
}

locals {
  resolved_db_password = var.db_admin_password != "" ? var.db_admin_password : random_password.db_password.result
  resolved_jwt_secret  = var.jwt_secret != "" ? var.jwt_secret : random_password.jwt_secret.result
}

resource "azurerm_key_vault" "kv" {
  name                            = substr("kv-${local.name_prefix}-${random_string.kv_suffix.result}", 0, 24)
  location                        = var.location
  resource_group_name             = var.resource_group_name
  tenant_id                       = var.tenant_id
  sku_name                        = "standard"
  enable_rbac_authorization       = true
  soft_delete_retention_days      = 7
  purge_protection_enabled        = var.environment == "production" ? true : false
  enabled_for_disk_encryption     = false
  enabled_for_deployment          = true
  enabled_for_template_deployment = true
  tags                            = local.common_tags

  network_acls {
    bypass         = "AzureServices"
    default_action = "Allow"
  }
}

resource "azurerm_key_vault_secret" "db_password" {
  name         = "database-password"
  value        = local.resolved_db_password
  key_vault_id = azurerm_key_vault.kv.id
  tags         = local.common_tags
}

resource "azurerm_key_vault_secret" "db_url" {
  count        = var.database_url != "" ? 1 : 0
  name         = "database-url"
  value        = var.database_url
  key_vault_id = azurerm_key_vault.kv.id
  tags         = local.common_tags
}

resource "azurerm_key_vault_secret" "gemini_api_key" {
  count        = var.gemini_api_key != "" ? 1 : 0
  name         = "gemini-api-key"
  value        = var.gemini_api_key
  key_vault_id = azurerm_key_vault.kv.id
  tags         = local.common_tags
}

resource "azurerm_key_vault_secret" "jwt_secret" {
  name         = "jwt-secret"
  value        = local.resolved_jwt_secret
  key_vault_id = azurerm_key_vault.kv.id
  tags         = local.common_tags
}

# Grant Container Apps Managed Identity "Key Vault Secrets User" role
resource "azurerm_role_assignment" "container_app_secrets_user" {
  count                = var.container_app_principal_id != null ? 1 : 0
  scope                = azurerm_key_vault.kv.id
  role_definition_name = "Key Vault Secrets User"
  principal_id         = var.container_app_principal_id
}

# Private Endpoint (optional / when subnet provided)
resource "azurerm_private_dns_zone" "vault" {
  count               = var.private_endpoints_subnet_id != null && var.vnet_id != null ? 1 : 0
  name                = "privatelink.vaultcore.azure.net"
  resource_group_name = var.resource_group_name
  tags                = local.common_tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "vault" {
  count                 = var.private_endpoints_subnet_id != null && var.vnet_id != null ? 1 : 0
  name                  = "vnet-link-kv-${local.name_prefix}"
  resource_group_name   = var.resource_group_name
  private_dns_zone_name = azurerm_private_dns_zone.vault[0].name
  virtual_network_id    = var.vnet_id
  tags                  = local.common_tags
}

resource "azurerm_private_endpoint" "kv" {
  count               = var.private_endpoints_subnet_id != null ? 1 : 0
  name                = "pe-kv-${local.name_prefix}"
  location            = var.location
  resource_group_name = var.resource_group_name
  subnet_id           = var.private_endpoints_subnet_id
  tags                = local.common_tags

  private_service_connection {
    name                           = "psc-kv-${local.name_prefix}"
    private_connection_resource_id = azurerm_key_vault.kv.id
    is_manual_connection           = false
    subresource_names              = ["vault"]
  }

  dynamic "private_dns_zone_group" {
    for_each = length(azurerm_private_dns_zone.vault) > 0 ? [1] : []
    content {
      name                 = "default"
      private_dns_zone_ids = [azurerm_private_dns_zone.vault[0].id]
    }
  }
}
