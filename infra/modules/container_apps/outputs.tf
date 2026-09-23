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
  description = "Stable app FQDN of the API (not a revision FQDN, which dies on the next deploy)"
  value       = azurerm_container_app.api.ingress[0].fqdn
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
  value       = azurerm_container_app.dashboard.ingress[0].fqdn
}

output "api_custom_domain_verification_id" {
  description = "Value for the API's asuid TXT record that proves custom domain ownership"
  value       = azurerm_container_app.api.custom_domain_verification_id
  sensitive   = true
}

output "dashboard_custom_domain_verification_id" {
  description = "Value for the dashboard's asuid TXT record that proves custom domain ownership"
  value       = azurerm_container_app.dashboard.custom_domain_verification_id
  sensitive   = true
}

output "metrics_token" {
  description = "Bearer token Prometheus must send to scrape /metrics"
  value       = random_password.metrics_token.result
  sensitive   = true
}
