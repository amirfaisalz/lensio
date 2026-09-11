mock_provider "azurerm" {
  mock_resource "azurerm_virtual_network" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.Network/virtualNetworks/vnet-nusaid-staging"
    }
  }
  mock_resource "azurerm_subnet" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.Network/virtualNetworks/vnet-nusaid-staging/subnets/snet"
    }
  }
  mock_resource "azurerm_network_security_group" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.Network/networkSecurityGroups/nsg"
    }
  }
  mock_resource "azurerm_key_vault" {
    defaults = {
      id        = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.KeyVault/vaults/kv-nusaid-staging"
      vault_uri = "https://kv-nusaid-staging.vault.azure.net/"
    }
  }
  mock_resource "azurerm_private_dns_zone" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.Network/privateDnsZones/nusaid-staging.postgres.database.azure.com"
    }
  }
  mock_resource "azurerm_postgresql_flexible_server" {
    defaults = {
      id   = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.DBforPostgreSQL/flexibleServers/psql-test"
      fqdn = "psql-test.postgres.database.azure.com"
    }
  }
  mock_resource "azurerm_log_analytics_workspace" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.OperationalInsights/workspaces/log-nusaid-staging"
    }
  }
  mock_resource "azurerm_user_assigned_identity" {
    defaults = {
      id           = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id-ca-nusaid-staging"
      principal_id = "11111111-2222-3333-4444-555555555555"
      client_id    = "66666666-7777-8888-9999-000000000000"
    }
  }
  mock_resource "azurerm_container_app_environment" {
    defaults = {
      id             = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.App/managedEnvironments/cae-nusaid-staging"
      default_domain = "southeastasia.azurecontainerapps.io"
    }
  }
  mock_resource "azurerm_container_app" {
    defaults = {
      id                   = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.App/containerApps/ca-app"
      latest_revision_fqdn = "ca-app.southeastasia.azurecontainerapps.io"
      latest_revision_name = "ca-app--rev1"
    }
  }
}

mock_provider "cloudflare" {}

run "validate_staging_composition" {
  command = plan

  assert {
    condition     = var.environment == "staging"
    error_message = "Staging environment must be configured"
  }

  assert {
    condition     = module.networking.resource_group_name == "rg-nusaid-staging"
    error_message = "Staging resource group name mismatch"
  }

  assert {
    condition     = module.postgres.admin_username == "nusaidadmin"
    error_message = "Postgres admin username mismatch"
  }
}
