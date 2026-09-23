mock_provider "azurerm" {
  mock_resource "azurerm_subnet" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.Network/virtualNetworks/vnet-lensio-staging/subnets/snet"
    }
  }
  mock_resource "azurerm_network_security_group" {
    defaults = {
      id = "/subscriptions/12345678-1234-9876-4563-123456789012/resourceGroups/rg-lensio-staging/providers/Microsoft.Network/networkSecurityGroups/nsg"
    }
  }
}

variables {
  project     = "lensio"
  environment = "staging"
  location    = "southeastasia"
}

run "validate_networking_resources" {
  command = plan

  assert {
    condition     = azurerm_resource_group.rg.name == "rg-lensio-staging"
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

  # Ingress terminates at 80/443; the container port (8080) is never reached directly.
  assert {
    condition     = toset(one(azurerm_network_security_group.container_apps.security_rule).destination_port_ranges) == toset(["80", "443"])
    error_message = "Container Apps NSG must only open 80 and 443 to the Internet"
  }
}
