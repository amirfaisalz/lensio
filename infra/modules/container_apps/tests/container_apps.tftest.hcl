mock_provider "azurerm" {
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
}

variables {
  project                  = "nusaid"
  environment              = "staging"
  location                 = "southeastasia"
  resource_group_name      = "rg-nusaid-staging"
  infrastructure_subnet_id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-nusaid-staging/providers/Microsoft.Network/virtualNetworks/vnet-nusaid-staging/subnets/snet-ca"
  database_url             = "postgres://nusaidadmin:secret@localhost:5432/nusaid?sslmode=require"
}

run "validate_container_apps_configuration" {
  command = plan

  assert {
    condition     = azurerm_container_app.api.name == "ca-api-nusaid-staging"
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
