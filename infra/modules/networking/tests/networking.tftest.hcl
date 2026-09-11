mock_provider "azurerm" {
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
}

variables {
  project     = "nusaid"
  environment = "staging"
  location    = "southeastasia"
}

run "validate_networking_resources" {
  command = plan

  assert {
    condition     = azurerm_resource_group.rg.name == "rg-nusaid-staging"
    error_message = "Resource group name did not match expected naming convention"
  }

  assert {
    condition     = azurerm_virtual_network.vnet.address_space[0] == "10.0.0.0/16"
    error_message = "VNet address space did not match default"
  }

  assert {
    condition     = azurerm_subnet.container_apps.address_prefixes[0] == "10.0.0.0/23"
    error_message = "Container Apps subnet CIDR did not match default"
  }

  assert {
    condition     = azurerm_subnet.postgres.address_prefixes[0] == "10.0.4.0/24"
    error_message = "PostgreSQL subnet CIDR did not match default"
  }

  assert {
    condition     = azurerm_subnet.private_endpoints.address_prefixes[0] == "10.0.5.0/24"
    error_message = "Private Endpoints subnet CIDR did not match default"
  }
}
