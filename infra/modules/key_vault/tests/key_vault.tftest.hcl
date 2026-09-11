mock_provider "azurerm" {
  mock_resource "azurerm_key_vault" {
    defaults = {
      id        = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.KeyVault/vaults/kv-lensio-staging"
      vault_uri = "https://kv-lensio-staging.vault.azure.net/"
    }
  }
}

variables {
  project             = "lensio"
  environment         = "staging"
  location            = "southeastasia"
  resource_group_name = "rg-lensio-staging"
  tenant_id           = "00000000-0000-0000-0000-000000000000"
}

run "validate_key_vault_configuration" {
  command = plan

  assert {
    condition     = azurerm_key_vault.kv.sku_name == "standard"
    error_message = "Key Vault SKU must be standard"
  }

  assert {
    condition     = azurerm_key_vault.kv.enable_rbac_authorization == true
    error_message = "Key Vault must enable RBAC authorization"
  }

  assert {
    condition     = azurerm_key_vault_secret.db_password.name == "database-password"
    error_message = "Secret database-password must be configured"
  }

  assert {
    condition     = azurerm_key_vault_secret.jwt_secret.name == "jwt-secret"
    error_message = "Secret jwt-secret must be configured"
  }
}
