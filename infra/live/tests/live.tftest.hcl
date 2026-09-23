# Run once per environment so the tfvars files themselves are checked:
#   tofu test -var-file=staging.tfvars
#   tofu test -var-file=production.tfvars
mock_provider "azurerm" {
  mock_resource "azurerm_virtual_network" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.Network/virtualNetworks/vnet-lensio"
    }
  }
  mock_resource "azurerm_subnet" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.Network/virtualNetworks/vnet-lensio/subnets/snet"
    }
  }
  mock_resource "azurerm_network_security_group" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.Network/networkSecurityGroups/nsg"
    }
  }
  mock_resource "azurerm_private_dns_zone" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.Network/privateDnsZones/lensio.postgres.database.azure.com"
    }
  }
  mock_resource "azurerm_postgresql_flexible_server" {
    defaults = {
      id   = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.DBforPostgreSQL/flexibleServers/psql-test"
      fqdn = "psql-test.postgres.database.azure.com"
    }
  }
  mock_resource "azurerm_log_analytics_workspace" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.OperationalInsights/workspaces/log-lensio"
    }
  }
  mock_resource "azurerm_container_app_environment" {
    defaults = {
      id             = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.App/managedEnvironments/cae-lensio"
      default_domain = "southeastasia.azurecontainerapps.io"
    }
  }
  mock_resource "azurerm_container_app" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio/providers/Microsoft.App/containerApps/ca-app"
    }
  }
}

run "validate_composition" {
  command = plan

  assert {
    condition     = module.networking.resource_group_name == "rg-lensio-${var.environment}"
    error_message = "Resource group name must follow rg-lensio-<environment>"
  }

  assert {
    condition     = module.postgres.admin_username == "lensioadmin"
    error_message = "Postgres admin username mismatch"
  }


  assert {
    condition     = var.environment != "production" || (var.postgres_ha_mode == "ZoneRedundant" && var.postgres_geo_redundant_backups && var.api_min_replicas >= 2)
    error_message = "Production needs zone-redundant HA, geo backups and at least 2 API replicas"
  }
}
