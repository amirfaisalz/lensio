mock_provider "azurerm" {
  mock_resource "azurerm_private_dns_zone" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.Network/privateDnsZones/lensio-staging.postgres.database.azure.com"
    }
  }
  mock_resource "azurerm_postgresql_flexible_server" {
    defaults = {
      id   = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.DBforPostgreSQL/flexibleServers/psql-test"
      fqdn = "psql-test.postgres.database.azure.com"
    }
  }
}

variables {
  project             = "lensio"
  environment         = "staging"
  location            = "southeastasia"
  resource_group_name = "rg-lensio-staging"
  vnet_id             = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.Network/virtualNetworks/vnet-lensio-staging"
  delegated_subnet_id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.Network/virtualNetworks/vnet-lensio-staging/subnets/snet-psql"
  admin_username      = "lensioadmin"
}

run "validate_postgres_configuration" {
  command = plan

  assert {
    condition     = azurerm_postgresql_flexible_server.postgres.version == "16"
    error_message = "PostgreSQL version must be 16"
  }

  assert {
    condition     = azurerm_postgresql_flexible_server.postgres.administrator_login == "lensioadmin"
    error_message = "Administrator login must match input"
  }

  assert {
    condition     = azurerm_postgresql_flexible_server_database.app_db.name == "lensio"
    error_message = "Database name must be lensio"
  }

  assert {
    condition     = azurerm_postgresql_flexible_server_configuration.require_secure_transport.value == "ON"
    error_message = "SSL secure transport must be ON"
  }

  assert {
    condition     = azurerm_postgresql_flexible_server.postgres.public_network_access_enabled == false
    error_message = "PostgreSQL must not be publicly reachable; it lives on the delegated subnet"
  }

  assert {
    condition     = azurerm_postgresql_flexible_server_configuration.connection_throttling.name == "connection_throttle.enable"
    error_message = "Connection throttling must use the Flexible Server parameter name"
  }

}

# apply (against the mocked azurerm) so the real random provider generates a
# password: it lands inside DATABASE_URL unescaped, where any URL delimiter
# would cut the URL short.
run "connection_string_is_url_safe" {
  command = apply

  assert {
    condition     = can(regex("^postgres://lensioadmin:[A-Za-z0-9]{32}@psql-test\\.postgres\\.database\\.azure\\.com:5432/lensio\\?sslmode=require$", output.connection_string))
    error_message = "connection_string must be a well-formed URL with a URL-safe password"
  }
}
