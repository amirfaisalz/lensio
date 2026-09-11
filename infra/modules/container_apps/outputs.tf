output "environment_id" {
  description = "ID of the Container App Environment"
  value       = azurerm_container_app_environment.cae.id
}

output "environment_default_domain" {
  description = "Default domain of the Container App Environment"
  value       = azurerm_container_app_environment.cae.default_domain
}

output "api_app_id" {
  description = "ID of the API Container App"
  value       = azurerm_container_app.api.id
}

output "api_fqdn" {
  description = "FQDN of the API Container App"
  value       = azurerm_container_app.api.latest_revision_fqdn
}

output "api_latest_revision_name" {
  description = "Latest revision name of the API Container App"
  value       = azurerm_container_app.api.latest_revision_name
}

output "dashboard_app_id" {
  description = "ID of the Dashboard Container App"
  value       = azurerm_container_app.dashboard.id
}

output "dashboard_fqdn" {
  description = "FQDN of the Dashboard Container App"
  value       = azurerm_container_app.dashboard.latest_revision_fqdn
}

output "identity_id" {
  description = "ID of User Assigned Identity"
  value       = azurerm_user_assigned_identity.ca_identity.id
}

output "identity_principal_id" {
  description = "Principal ID of User Assigned Identity"
  value       = azurerm_user_assigned_identity.ca_identity.principal_id
}

output "identity_client_id" {
  description = "Client ID of User Assigned Identity"
  value       = azurerm_user_assigned_identity.ca_identity.client_id
}
