output "resource_group_name" {
  description = "Name of the created resource group"
  value       = azurerm_resource_group.rg.name
}

output "resource_group_id" {
  description = "ID of the created resource group"
  value       = azurerm_resource_group.rg.id
}

output "location" {
  description = "Azure location"
  value       = azurerm_resource_group.rg.location
}

output "vnet_id" {
  description = "ID of the virtual network"
  value       = azurerm_virtual_network.vnet.id
}

output "vnet_name" {
  description = "Name of the virtual network"
  value       = azurerm_virtual_network.vnet.name
}

output "container_apps_subnet_id" {
  description = "ID of the subnet delegated to Container Apps"
  value       = azurerm_subnet.container_apps.id
}

output "postgres_subnet_id" {
  description = "ID of the subnet delegated to PostgreSQL Flexible Server"
  value       = azurerm_subnet.postgres.id
}
