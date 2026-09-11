mock_provider "azurerm" {
  mock_resource "azurerm_log_analytics_workspace" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.OperationalInsights/workspaces/log-lensio-staging"
    }
  }
  mock_resource "azurerm_user_assigned_identity" {
    defaults = {
      id           = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id-ca-lensio-staging"
      principal_id = "11111111-2222-3333-4444-555555555555"
      client_id    = "66666666-7777-8888-9999-000000000000"
    }
  }
  mock_resource "azurerm_container_app_environment" {
    defaults = {
      id             = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.App/managedEnvironments/cae-lensio-staging"
      default_domain = "southeastasia.azurecontainerapps.io"
    }
  }
}

variables {
  project                  = "lensio"
  environment              = "staging"
  location                 = "southeastasia"
  resource_group_name      = "rg-lensio-staging"
  infrastructure_subnet_id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.Network/virtualNetworks/vnet-lensio-staging/subnets/snet-ca"
  database_url             = "postgres://lensioadmin:secret@localhost:5432/lensio?sslmode=require"
}

run "validate_container_apps_configuration" {
  command = plan

  assert {
    condition     = azurerm_container_app.api.name == "ca-api-lensio-staging"
    error_message = "API container app name must match convention"
  }

  assert {
    condition     = azurerm_container_app.api.ingress[0].target_port == 8080
    error_message = "API target port must be 8080"
  }

  assert {
    condition     = azurerm_container_app.dashboard.ingress[0].target_port == 80
    error_message = "Dashboard target port must be 80"
  }
}
