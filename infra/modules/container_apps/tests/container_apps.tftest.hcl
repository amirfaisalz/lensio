mock_provider "azurerm" {
  mock_resource "azurerm_log_analytics_workspace" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.OperationalInsights/workspaces/log-lensio-staging"
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

  # Rollback shifts traffic between revisions, which Single mode forbids.
  assert {
    condition     = azurerm_container_app.api.revision_mode == "Multiple" && azurerm_container_app.dashboard.revision_mode == "Multiple"
    error_message = "Both apps must run in Multiple revision mode so rollback can shift traffic"
  }

  # The API exits on boot in production/staging without these.
  assert {
    condition = alltrue([for name in ["SESSION_SECRET", "METRICS_TOKEN", "DATABASE_URL", "OCR_PROVIDER", "CORS_ALLOWED_ORIGINS", "SMTP_HOST"] :
    contains([for e in azurerm_container_app.api.template[0].container[0].env : e.name], name)])
    error_message = "API container is missing an environment variable it needs to boot"
  }

  assert {
    condition     = length(azurerm_container_app.api.registry) == 0
    error_message = "No registry block expected when no registry password is supplied"
  }
}

# Origins derive from the environment's default domain (mocked above), so they
# are known at plan time.
run "no_custom_domain_uses_azure_origins" {
  command = plan

  assert {
    condition = alltrue([
      for e in azurerm_container_app.api.template[0].container[0].env :
      e.value == "https://ca-dash-lensio-staging.southeastasia.azurecontainerapps.io"
      if contains(["CORS_ALLOWED_ORIGINS", "APP_BASE_URL"], e.name)
    ])
    error_message = "Without a custom domain, CORS_ALLOWED_ORIGINS and APP_BASE_URL must be the dashboard's Azure origin"
  }

  # The dashboard reaches the API directly, so it must be told where it is.
  assert {
    condition = one([
      for e in azurerm_container_app.dashboard.template[0].container[0].env : e.value if e.name == "API_URL"
    ]) == "https://ca-api-lensio-staging.southeastasia.azurecontainerapps.io"
    error_message = "Dashboard API_URL must default to the API's Azure origin"
  }
}

run "custom_domains" {
  command = plan

  variables {
    api_public_url       = "https://api.lensio.tec.my.id"
    cors_allowed_origins = ["https://lensio.tec.my.id"]
    app_base_url         = "https://lensio.tec.my.id"
  }

  # The Azure origin stays allowed next to the custom domain.
  assert {
    condition = one([
      for e in azurerm_container_app.api.template[0].container[0].env : e.value if e.name == "CORS_ALLOWED_ORIGINS"
    ]) == "https://ca-dash-lensio-staging.southeastasia.azurecontainerapps.io,https://lensio.tec.my.id"
    error_message = "CORS must allow both the dashboard's Azure origin and its custom domain"
  }

  assert {
    condition = one([
      for e in azurerm_container_app.api.template[0].container[0].env : e.value if e.name == "APP_BASE_URL"
    ]) == "https://lensio.tec.my.id"
    error_message = "An explicit app_base_url must win over the Azure origin"
  }

  assert {
    condition = one([
      for e in azurerm_container_app.dashboard.template[0].container[0].env : e.value if e.name == "API_URL"
    ]) == "https://api.lensio.tec.my.id"
    error_message = "An explicit api_public_url must win over the API's Azure origin"
  }
}

run "private_registry_credentials" {
  command = plan

  variables {
    registry_username = "amirfaisalz"
    registry_password = "ghp_synthetic"
  }

  assert {
    condition     = azurerm_container_app.api.registry[0].server == "ghcr.io" && azurerm_container_app.dashboard.registry[0].password_secret_name == "registry-password"
    error_message = "Both apps must pull from GHCR with the registry-password secret"
  }
}
